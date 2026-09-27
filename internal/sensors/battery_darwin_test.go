//go:build darwin

package sensors

import (
	"errors"
	"testing"
)

// TestBatteryReadsRealHardware reads the real AppleSmartBattery, so it skips
// under -short and on desktop Macs, and only checks that values are sane —
// never exact. Health and TempC guard against a macOS release moving the keys
// again (macOS 27 moved them into AppleSmartBatteryPack).
func TestBatteryReadsRealHardware(t *testing.T) {
	if testing.Short() {
		t.Skip("reads real battery state")
	}

	st, err := NewBattery().Battery()
	if errors.Is(err, errBatteryUnavailable) {
		t.Skip("no battery on this Mac")
	}

	if err != nil {
		t.Fatalf("Battery() error: %v", err)
	}

	if st.Percent <= 0 || st.Percent > 1 {
		t.Errorf("Percent = %v, want (0, 1]", st.Percent)
	}

	if st.RawPercent <= 0 {
		t.Errorf("RawPercent = %v, want > 0", st.RawPercent)
	}

	if st.Health <= 0 {
		t.Errorf("Health = %v, want > 0", st.Health)
	}

	if st.TempC <= 0 {
		t.Errorf("TempC = %v, want > 0", st.TempC)
	}
}
