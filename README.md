# pulse

Нативный menu bar монитор для macOS — аналог GNOME-расширения
[Vitals](https://github.com/corecoding/Vitals) по фичам (не по коду). Go +
CGO (Mach API), без sudo и без `powermetrics`.

## Статус: v0.1

- [x] Загрузка CPU: общая + по ядрам (`host_processor_info`, Mach)
- [x] Память: used / available / physical / swap (`host_statistics64` + sysctl)
- [x] Menu bar: `CPU 7%  24.3G` инлайн, полный список в дропдауне
- [x] Определение чипа (Intel / Apple Silicon) в рантайме
- [ ] Температуры (SMC на Intel, IOHIDEventSystemClient на Apple Silicon)
- [ ] Вентиляторы, сеть, диск, питание (IOReport), настройки

## Сборка и запуск

Нужны macOS и Xcode command line tools.

```sh
make run    # собрать Pulse.app, подписать ad-hoc и запустить
make once   # проверить сенсоры без UI: один кадр метрик в stdout
make test   # юнит-тесты
```

`Pulse.app` — background-агент (`LSUIElement=true`): иконки в Dock нет,
только пункт в menu bar. Выход — через «Quit pulse» в дропдауне.

## Структура

Слои по мотивам [go-clean-template](https://github.com/evrone/go-clean-template):
`internal/sensors` (источники данных, аналог repo) → `internal/usecase`
(сэмплирование, дельты CPU) → `internal/controller/tray` (systray UI);
доменные типы в `internal/entity`, форматирование значений в стиле Vitals —
в `pkg/format`.

## Протестировано на

| Модель | Чип | macOS | Фичи |
|---|---|---|---|
| Mac17,8 (MacBook Pro) | Apple M5 Pro, 18 ядер | 26.5.2 | CPU, память |
