package systray

import "strings"

// PATCH(pulse): attributed status-item titles with inline template icons.

// TitlePart is one segment of the status item title: an optional icon
// (registered via RegisterTitleIcon) followed by text. Icons are only
// rendered on macOS; other platforms fall back to the concatenated text.
type TitlePart struct {
	Icon string // key registered via RegisterTitleIcon; "" = no icon
	Text string
}

// Control characters used to encode parts across the C boundary; they are
// stripped from user text so the encoding cannot be broken.
const (
	titlePartSep  = "\x1e"
	titleFieldSep = "\x1f"
)

var titleCleaner = strings.NewReplacer(titlePartSep, "", titleFieldSep, "")

func encodeTitleParts(parts []TitlePart) string {
	segs := make([]string, 0, len(parts))
	for _, p := range parts {
		segs = append(segs, titleCleaner.Replace(p.Icon)+titleFieldSep+titleCleaner.Replace(p.Text))
	}
	return strings.Join(segs, titlePartSep)
}
