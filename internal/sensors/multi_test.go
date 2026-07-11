package sensors

import (
	"fmt"
	"testing"

	"github.com/emgeorrk/pulse/internal/entity"
)

func TestMultiTemp(t *testing.T) {
	hid := TempFunc(func() ([]entity.Reading, error) {
		return []entity.Reading{{Name: "PMU tdie1", Value: 35}}, nil
	})
	gpu := TempFunc(func() ([]entity.Reading, error) {
		return []entity.Reading{{Name: "GPU die", Value: 42}}, nil
	})
	empty := TempFunc(func() ([]entity.Reading, error) { return nil, nil })
	fail := TempFunc(func() ([]entity.Reading, error) { return nil, fmt.Errorf("boom") })

	cases := []struct {
		name    string
		sources []TempSource
		want    []string // reading names in order; nil means expect an error
	}{
		{"both ok, argument order kept", []TempSource{hid, gpu}, []string{"PMU tdie1", "GPU die"}},
		{"first fails, second still returned", []TempSource{fail, gpu}, []string{"GPU die"}},
		{"second fails, first still returned", []TempSource{hid, fail}, []string{"PMU tdie1"}},
		{"empty-but-successful source is fine", []TempSource{empty, gpu}, []string{"GPU die"}},
		{"all fail", []TempSource{fail, fail}, nil},
	}
	for _, c := range cases {
		got, err := NewMultiTemp(c.sources...).Temps()
		if c.want == nil {
			if err == nil {
				t.Errorf("%s: want error, got %v", c.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d readings, want %d", c.name, len(got), len(c.want))
			continue
		}
		for i, name := range c.want {
			if got[i].Name != name {
				t.Errorf("%s: reading %d = %q, want %q", c.name, i, got[i].Name, name)
			}
		}
	}
}
