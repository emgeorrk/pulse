# pulse

Нативный menu bar монитор для macOS — аналог GNOME-расширения
[Vitals](https://github.com/corecoding/Vitals) по фичам (не по коду). Go +
CGO, без sudo и без `powermetrics`.

## Фичи

- **CPU**: общая загрузка + по ядрам, спарклайн истории (▁▂▄▇) в menu bar,
  частота по кластерам (IOReport performance states × таблицы частот из
  device tree)
- **Память**: used / available / physical / swap (формула Activity Monitor)
- **Температуры**: все сенсоры чипа + агрегаты CPU/GPU/hottest
  (Apple Silicon — HID sensor hub; Intel — SMC-ключи)
- **Вентиляторы**: обороты + % от максимума (SMC; на безвентиляторных
  моделях группа скрыта)
- **Вольтаж**: сенсоры PMU (Apple Silicon)
- **Сеть**: суммарные ↓/↑ и по интерфейсам, трафик за сессию
- **Диск**: заполненность тома, скорости чтения/записи, тоталы с загрузки
- **GPU**: загрузка (IOAccelerator)
- **Мощность**: CPU/GPU/ANE в ваттах (IOReport Energy Model)
- **Батарея**: заряд, health, циклы, температура, вольтаж, ватты, время

## UI (модель Vitals)

- Пиннутые метрики — инлайн в menu bar; клик по метрике в дропдауне
  (чекбокс) пинит/отпинивает её, порядок пиннинга = порядок в баре
- Группы в дропдауне показывают живой агрегат прямо в заголовке
- **Settings**: интервал (1/2/3/5 с, без рестарта), °C/°F, GiB/GB,
  спарклайн, автозапуск при логине (LaunchAgent)
- Настройки в `~/Library/Application Support/pulse/config.json`

## Сборка и запуск

Нужны macOS и Xcode command line tools.

```sh
make run    # собрать Pulse.app, подписать ad-hoc и запустить
make once   # проверить сенсоры без UI: один кадр метрик в stdout
make test   # юнит-тесты
```

`PULSE_DEBUG=1 ./bin/pulse -once` дополнительно печатает каналы IOReport —
пригодится при портировании на новое поколение чипов.

`Pulse.app` — background-агент (`LSUIElement=true`): иконки в Dock нет,
только пункт в menu bar. Выход — «Quit pulse» в дропдауне.

## Структура

Слои по мотивам [go-clean-template](https://github.com/evrone/go-clean-template):
`internal/sensors` (источники данных: Mach, getifaddrs, IOKit, SMC, HID,
IOReport) → `internal/usecase` (сэмплирование, дельты, агрегаты) →
`internal/controller/tray` (systray UI, реестр метрик); доменные типы в
`internal/entity`, форматирование в стиле Vitals — в `pkg/format`.

Недоступный на данном железе сенсор выключает свою группу (Caps), а не
приложение.

## Платформенные пути

| Метрика | Apple Silicon | Intel |
|---|---|---|
| Температуры | `IOHIDEventSystemClient` (0xff00/5) | SMC-ключи (TC0P…) — **untested** |
| Вольтаж | HID (0xff08/3) | — |
| Вентиляторы | SMC `F%dAc`, тип `flt ` LE | SMC, тип `fpe2` BE — **untested** |
| Мощность | IOReport Energy Model | — |
| Частота CPU | IOReport perf states | — |

## Протестировано на

| Модель | Чип | macOS | Фичи |
|---|---|---|---|
| Mac17,8 (MacBook Pro) | Apple M5 Pro, 18 ядер | 26.5.2 | всё, кроме Intel-путей |

Intel-пути (SMC-температуры sp78, вентиляторы fpe2) написаны по референсам
(iSMC, VirtualSMC docs) и на реальном Intel-железе не проверялись.
