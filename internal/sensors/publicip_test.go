package sensors

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emgeorrk/pulse/internal/entity"
)

// testProvider pairs a fake endpoint with the parser the real provider list
// would use for it.
type testProvider struct {
	handler http.HandlerFunc
	parse   func([]byte) (entity.PublicIPInfo, error)
}

func TestPublicIPFetch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []testProvider
		want      entity.PublicIPInfo
		wantErr   error
	}{
		{
			name: "geo provider answers with ip and country",
			providers: []testProvider{
				{
					handler: func(w http.ResponseWriter, _ *http.Request) {
						fmt.Fprint(w, `{"ip":"203.0.113.7","country":"Netherlands","cc":"NL"}`)
					},
					parse: parseMyIP,
				},
			},
			want: entity.PublicIPInfo{IP: "203.0.113.7", Country: "NL"},
		},
		{
			name: "fallback geo provider after error status",
			providers: []testProvider{
				{
					handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
					parse:   parseMyIP,
				},
				{
					handler: func(w http.ResponseWriter, _ *http.Request) {
						fmt.Fprint(w, `{"ip":"2001:db8::1","country":"ie"}`)
					},
					parse: parseCountryIs,
				},
			},
			want: entity.PublicIPInfo{IP: "2001:db8::1", Country: "IE"},
		},
		{
			name: "unknown country marker yields ip only",
			providers: []testProvider{
				{
					handler: func(w http.ResponseWriter, _ *http.Request) {
						fmt.Fprint(w, `{"ip":"203.0.113.7","country":"","cc":"XX"}`)
					},
					parse: parseMyIP,
				},
			},
			want: entity.PublicIPInfo{IP: "203.0.113.7"},
		},
		{
			name: "broken json falls through to plain provider",
			providers: []testProvider{
				{
					handler: func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"ip":`) },
					parse:   parseMyIP,
				},
				{
					handler: func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "203.0.113.7\n") },
					parse:   parsePlainIP,
				},
			},
			want: entity.PublicIPInfo{IP: "203.0.113.7"},
		},
		{
			name: "non-address body is rejected",
			providers: []testProvider{
				{
					handler: func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "<html>nope</html>") },
					parse:   parsePlainIP,
				},
			},
			wantErr: errPublicIPInvalid,
		},
		{
			name: "error status from every provider",
			providers: []testProvider{
				{
					handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
					parse:   parseMyIP,
				},
			},
			wantErr: errPublicIPStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &PublicIP{client: &http.Client{Timeout: time.Second}}
			for _, tp := range tt.providers {
				srv := httptest.NewServer(tp.handler)
				t.Cleanup(srv.Close)
				p.providers = append(p.providers, ipProvider{url: srv.URL, parse: tp.parse})
			}

			got, err := p.Fetch(context.Background())
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Fetch() error = %v, want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("Fetch() error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Fetch() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
