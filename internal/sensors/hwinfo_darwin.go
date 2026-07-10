//go:build darwin

package sensors

import (
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/emgeorrk/pulse/internal/entity"
)

// ReadHWInfo определяет чип, модель Mac и версию macOS. Ошибки sysctl не
// фатальны — недостающие поля остаются пустыми, UI их просто не покажет.
func ReadHWInfo() entity.HWInfo {
	chip, _ := unix.Sysctl("machdep.cpu.brand_string")
	model, _ := unix.Sysctl("hw.model")
	osVer, _ := unix.Sysctl("kern.osproductversion")

	chip = strings.TrimSpace(chip)
	if chip == "" {
		chip = runtime.GOARCH
	}

	return entity.HWInfo{
		Chip:           chip,
		Model:          model,
		OSVersion:      osVer,
		IsAppleSilicon: strings.Contains(chip, "Apple"),
		NumCores:       runtime.NumCPU(),
	}
}
