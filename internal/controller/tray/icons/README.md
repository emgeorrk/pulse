# Metric icons

The SVG sources in `svg/` are the "GNOME" icon theme of the
[Vitals](https://github.com/corecoding/Vitals) GNOME Shell extension
(`icons/gnome/*-symbolic.svg`), copied verbatim. The PNGs in `png/` are
rendered from them by `scripts/gen-icons.sh` (64x64, glyph + alpha) and are
what the app embeds; the app draws them template-rendered (menu items) or
tinted to the menu bar text color (title), so only the alpha channel is used.

## Provenance and license

Vitals is licensed under **GPL-2.0**. Per its README, the GNOME icon set
originates from:

- `battery`, `storage` — [Adwaita Icon Theme](https://gitlab.gnome.org/GNOME/adwaita-icon-theme)
- `memory`, `network*`, `system`, `voltage` — GNOME [Icon Development Kit](https://gitlab.gnome.org/Teams/Design/icon-development-kit)
- `fan` — inherited from the Freon extension (modified)
- `temperature`, `cpu` — designed by [daudix](https://daudix.github.io)

`settings.png` is not from Vitals: it is Apple's SF Symbol `gearshape.fill`
rendered by `scripts/sfsymbol2png.swift` (the Vitals set has no gear). SF
Symbols may be used in apps for Apple platforms per Apple's license terms.

Redistributing Pulse with these icons embedded carries the corresponding
GPL/attribution obligations. Keep this notice when distributing.
