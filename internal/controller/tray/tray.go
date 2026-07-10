//go:build darwin

// Package tray — menu bar UI на fyne.io/systray по модели Vitals: пиннутые
// метрики инлайн в title, полный список в дропдауне по группам, пиннинг
// кликом-чекбоксом, настройки в подменю Settings.
package tray

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"fyne.io/systray"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/autostart"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/pkg/format"
)

type groupUI struct {
	group
	item *systray.MenuItem
	rows []*systray.MenuItem // параллелен group.metrics
}

type Tray struct {
	store *config.Store
	hw    entity.HWInfo

	groups []groupUI
	bar    map[entity.MetricID]metric // метрики по id для рендера в menu bar

	mu   sync.Mutex
	last entity.Snapshot
	seen bool // получен ли хоть один кадр
}

func New(store *config.Store, hw entity.HWInfo, caps entity.Caps) *Tray {
	t := &Tray{store: store, hw: hw, bar: map[entity.MetricID]metric{}}
	for _, g := range buildGroups(hw, caps) {
		t.groups = append(t.groups, groupUI{group: g})
		for _, m := range g.metrics {
			t.bar[m.id] = m
		}
	}
	return t
}

// Run блокирует до выхода из приложения. systray.Run обязан выполняться на
// главной горутине (Cocoa main thread); start вызывается уже из onReady.
func (t *Tray) Run(start func(ctx context.Context) <-chan entity.Snapshot) {
	ctx, cancel := context.WithCancel(context.Background())
	systray.Run(func() {
		t.build()
		go t.consume(start(ctx))
	}, cancel)
}

func (t *Tray) build() {
	systray.SetTitle("pulse …")
	systray.SetTooltip("pulse — system monitor")

	cfg := t.store.Get()

	for gi := range t.groups {
		g := &t.groups[gi]
		g.item = systray.AddMenuItem(g.emoji+" "+g.label, "")
		for _, m := range g.metrics {
			it := g.item.AddSubMenuItemCheckbox(m.label+": —", "", cfg.IsPinned(m.id))
			it.KeepMenuOpen() // пиннинг нескольких метрик за одно открытие меню
			go t.watchPin(m.id, it)
			g.rows = append(g.rows, it)
		}
	}

	systray.AddSeparator()
	sys := systray.AddMenuItem("ℹ️ System", "")
	sys.AddSubMenuItem(t.hw.Chip, "")
	switch {
	case t.hw.ModelName != "":
		sys.AddSubMenuItem(prettyModel(t.hw.ModelName, t.hw.Chip), "")
	case t.hw.Model != "":
		sys.AddSubMenuItem("Model: "+t.hw.Model, "")
	}
	if t.hw.OSVersion != "" {
		sys.AddSubMenuItem("macOS "+t.hw.OSVersion, "")
	}

	t.buildSettings(cfg)

	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit pulse", "")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()
}

func (t *Tray) buildSettings(cfg config.Config) {
	s := systray.AddMenuItem("🛠️ Settings", "")

	// интервал обновления — радиогруппа
	intervals := []int{1, 2, 3, 5}
	var intervalItems []*systray.MenuItem
	for _, sec := range intervals {
		sec := sec
		label := formatSeconds(sec)
		it := s.AddSubMenuItemCheckbox("Update: "+label, "", cfg.IntervalSec == sec)
		it.KeepMenuOpen()
		intervalItems = append(intervalItems, it)
		go func(me *systray.MenuItem) {
			for range me.ClickedCh {
				t.store.Update(func(c *config.Config) { c.IntervalSec = sec })
				for _, other := range intervalItems {
					other.Uncheck()
				}
				me.Check()
			}
		}(it)
	}

	// единицы температуры — радиогруппа (актуально с этапа температур)
	tempC := s.AddSubMenuItemCheckbox("Temperature: °C", "", cfg.TempUnit == config.Celsius)
	tempF := s.AddSubMenuItemCheckbox("Temperature: °F", "", cfg.TempUnit == config.Fahrenheit)
	tempC.KeepMenuOpen()
	tempF.KeepMenuOpen()
	go t.watchRadio(tempC, tempF, func(c *config.Config) { c.TempUnit = config.Celsius })
	go t.watchRadio(tempF, tempC, func(c *config.Config) { c.TempUnit = config.Fahrenheit })

	// единицы памяти — радиогруппа
	binU := s.AddSubMenuItemCheckbox("Memory: GiB (binary)", "", !cfg.DecimalBytes)
	decU := s.AddSubMenuItemCheckbox("Memory: GB (decimal)", "", cfg.DecimalBytes)
	binU.KeepMenuOpen()
	decU.KeepMenuOpen()
	go t.watchRadio(binU, decU, func(c *config.Config) { c.DecimalBytes = false })
	go t.watchRadio(decU, binU, func(c *config.Config) { c.DecimalBytes = true })

	// спарклайн CPU в menu bar
	spark := s.AddSubMenuItemCheckbox("CPU sparkline in bar", "", cfg.ShowSparkline)
	spark.KeepMenuOpen()
	go func() {
		for range spark.ClickedCh {
			var on bool
			t.store.Update(func(c *config.Config) { c.ShowSparkline = !c.ShowSparkline; on = c.ShowSparkline })
			setChecked(spark, on)
			t.refresh()
		}
	}()

	// автозапуск при логине
	login := s.AddSubMenuItemCheckbox("Start at login", "", cfg.StartAtLogin)
	login.KeepMenuOpen()
	go func() {
		for range login.ClickedCh {
			var on bool
			t.store.Update(func(c *config.Config) { c.StartAtLogin = !c.StartAtLogin; on = c.StartAtLogin })
			var err error
			if on {
				err = autostart.Enable()
			} else {
				err = autostart.Disable()
			}
			if err != nil { // откат: не смогли записать LaunchAgent — не врём чекбоксом
				t.store.Update(func(c *config.Config) { c.StartAtLogin = !on })
				on = !on
			}
			setChecked(login, on)
		}
	}()
}

// watchRadio: клик по me включает его, выключает other и применяет apply.
func (t *Tray) watchRadio(me, other *systray.MenuItem, apply func(*config.Config)) {
	for range me.ClickedCh {
		t.store.Update(apply)
		me.Check()
		other.Uncheck()
		t.refresh()
	}
}

func (t *Tray) watchPin(id entity.MetricID, item *systray.MenuItem) {
	for range item.ClickedCh {
		setChecked(item, t.store.TogglePin(id))
		t.refresh()
	}
}

func (t *Tray) consume(ch <-chan entity.Snapshot) {
	for snap := range ch {
		t.mu.Lock()
		t.last = snap
		t.seen = true
		t.mu.Unlock()
		t.render(snap)
	}
}

// refresh перерисовывает UI по последнему кадру — для мгновенной реакции на
// пиннинг/настройки, не дожидаясь следующего тика.
func (t *Tray) refresh() {
	t.mu.Lock()
	snap, ok := t.last, t.seen
	t.mu.Unlock()
	if ok {
		t.render(snap)
	}
}

func (t *Tray) render(s entity.Snapshot) {
	cfg := t.store.Get()
	systray.SetTitle(t.title(s, cfg))

	for gi := range t.groups {
		g := &t.groups[gi]
		if g.aggregate != nil {
			g.item.SetTitle(g.emoji + " " + g.label + " · " + g.aggregate(s, cfg))
		}
		for i, m := range g.metrics {
			g.rows[i].SetTitle(m.label + ": " + m.menu(s, cfg))
		}
	}
}

func (t *Tray) title(s entity.Snapshot, cfg config.Config) string {
	var parts []string
	if cfg.ShowSparkline && len(s.CPU.History) > 0 {
		parts = append(parts, format.Sparkline(s.CPU.History))
	}
	for _, id := range cfg.Pinned {
		if m, ok := t.bar[id]; ok {
			parts = append(parts, m.barText(s, cfg))
		}
	}
	if len(parts) == 0 {
		return "pulse"
	}
	return strings.Join(parts, "  ")
}

func setChecked(item *systray.MenuItem, on bool) {
	if on {
		item.Check()
	} else {
		item.Uncheck()
	}
}

func formatSeconds(sec int) string {
	return fmt.Sprintf("%d s", sec)
}

// prettyModel убирает из product-name скобки и дубль чипа — он уже показан
// отдельной строкой: "MacBook Pro (16-inch, M5 Pro)" → "MacBook Pro 16-inch".
func prettyModel(name, chip string) string {
	base, rest, ok := strings.Cut(name, "(")
	if !ok {
		return name
	}
	parts := []string{strings.TrimSpace(base)}
	rest = strings.TrimSuffix(strings.TrimSpace(rest), ")")
	for _, tok := range strings.Split(rest, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" || strings.Contains(chip, tok) {
			continue
		}
		parts = append(parts, tok)
	}
	return strings.Join(parts, " ")
}
