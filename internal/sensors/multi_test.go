package sensors_test

import (
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
	"github.com/emgeorrk/pulse/internal/sensors/mocks"
)

func TestMultiTemp(t *testing.T) {
	t.Parallel()

	// One entry per TempSource passed to NewMultiTemp, in argument order.
	type sourceResult struct {
		readings []entity.Reading
		err      error
	}

	hid := sourceResult{readings: []entity.Reading{{Name: "PMU tdie1", Value: 35}}}
	gpu := sourceResult{readings: []entity.Reading{{Name: "GPU die", Value: 42}}}
	empty := sourceResult{}
	fail := sourceResult{err: fmt.Errorf("boom")}

	tests := []struct {
		name    string
		sources []sourceResult
		want    []string // reading names in order; nil means expect an error
	}{
		{"both ok, argument order kept", []sourceResult{hid, gpu}, []string{"PMU tdie1", "GPU die"}},
		{"first fails, second still returned", []sourceResult{fail, gpu}, []string{"GPU die"}},
		{"second fails, first still returned", []sourceResult{hid, fail}, []string{"PMU tdie1"}},
		{"empty-but-successful source is fine", []sourceResult{empty, gpu}, []string{"GPU die"}},
		{"all fail", []sourceResult{fail, fail}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			srcs := make([]sensors.TempSource, 0, len(tt.sources))
			for _, sr := range tt.sources {
				m := mocks.NewMockTempSource(ctrl)
				m.EXPECT().Temps().Return(sr.readings, sr.err)
				srcs = append(srcs, m)
			}

			got, err := sensors.NewMultiTemp(srcs...).Temps()
			if tt.want == nil {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d readings, want %d", len(got), len(tt.want))
			}

			for i, name := range tt.want {
				if got[i].Name != name {
					t.Errorf("reading %d = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}
