package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
	"github.com/emgeorrk/pulse/internal/sensors/mocks"
	"go.uber.org/mock/gomock"
)

func TestPublicIPDue(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		fetchedAt time.Time
		name      string
		lastOK    bool
		want      bool
	}{
		{name: "never fetched", want: true},
		{name: "fresh success", fetchedAt: now.Add(-time.Minute), lastOK: true, want: false},
		{name: "stale success", fetchedAt: now.Add(-16 * time.Minute), lastOK: true, want: true},
		{name: "recent failure", fetchedAt: now.Add(-30 * time.Second), lastOK: false, want: false},
		{name: "failure past retry", fetchedAt: now.Add(-2 * time.Minute), lastOK: false, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := publicIPDue(tt.fetchedAt, tt.lastOK, now); got != tt.want {
				t.Errorf("publicIPDue(%v, %t) = %t, want %t", tt.fetchedAt, tt.lastOK, got, tt.want)
			}
		})
	}
}

func TestPollPublicIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expect      func(src *mocks.MockPublicIPSource)
		name        string
		wantIP      string
		wantCountry string
		enable      bool
	}{
		{
			name:   "disabled setting starts no lookup",
			enable: false,
			expect: func(*mocks.MockPublicIPSource) {}, // any Fetch call would fail the mock
			wantIP: "",
		},
		{
			name:   "enabled setting fetches once and keeps address and country",
			enable: true,
			expect: func(src *mocks.MockPublicIPSource) {
				src.EXPECT().Fetch(gomock.Any()).
					Return(entity.PublicIPInfo{IP: "203.0.113.7", Country: "NL"}, nil)
			},
			wantIP:      "203.0.113.7",
			wantCountry: "NL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			src := mocks.NewMockPublicIPSource(ctrl)
			tt.expect(src)

			store := config.Load("") // in-memory settings
			store.Update(func(c *config.Config) { c.ShowPublicIP = tt.enable })

			m := NewMonitor(&sensors.Sources{PublicIP: src}, store)
			m.pollPublicIP(context.Background())

			if tt.wantIP == "" {
				if m.ip != "" {
					t.Fatalf("ip = %q, want empty", m.ip)
				}

				return
			}

			// The lookup runs in its own goroutine and lands on the next
			// poll; a poll on a fresh value must not start another fetch
			// (the mock allows exactly one call).
			deadline := time.Now().Add(2 * time.Second)
			for m.ip == "" && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
				m.pollPublicIP(context.Background())
			}

			if m.ip != tt.wantIP {
				t.Errorf("ip = %q, want %q", m.ip, tt.wantIP)
			}

			if m.ipCountry != tt.wantCountry {
				t.Errorf("ipCountry = %q, want %q", m.ipCountry, tt.wantCountry)
			}
		})
	}
}
