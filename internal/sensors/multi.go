package sensors

import "github.com/emgeorrk/pulse/internal/entity"

// TempFunc adapts a plain function to TempSource (like http.HandlerFunc).
type TempFunc func() ([]entity.Reading, error)

func (f TempFunc) Temps() ([]entity.Reading, error) { return f() }

// MultiTemp merges readings from several TempSources (Apple Silicon: HID
// sensors plus the SMC GPU keys). A failing source is skipped; it errors
// only when every source fails.
type MultiTemp struct {
	sources []TempSource
}

func NewMultiTemp(srcs ...TempSource) *MultiTemp {
	return &MultiTemp{sources: srcs}
}

// Temps appends successful sources' readings in argument order, keeping the
// sensor ordering stable across ticks.
func (m *MultiTemp) Temps() ([]entity.Reading, error) {
	var (
		merged   []entity.Reading
		firstErr error
		ok       bool
	)
	for _, s := range m.sources {
		readings, err := s.Temps()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		ok = true
		merged = append(merged, readings...)
	}
	if !ok {
		return nil, firstErr
	}
	return merged, nil
}
