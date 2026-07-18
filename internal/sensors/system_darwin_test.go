//go:build darwin

package sensors

import "testing"

// TestSystemReadsRealHardware reads real kernel state, so it skips under
// -short and only checks that values are sane — never exact.
func TestSystemReadsRealHardware(t *testing.T) {
	if testing.Short() {
		t.Skip("reads real kernel state")
	}

	st, err := NewSystem().System()
	if err != nil {
		t.Fatalf("System() error: %v", err)
	}

	if st.Load1 < 0 || st.Load5 < 0 || st.Load15 < 0 {
		t.Errorf("negative load averages: %+v", st)
	}

	if st.UptimeSec == 0 {
		t.Error("uptime is zero")
	}

	if st.Procs <= 0 {
		t.Errorf("Procs = %d, want > 0", st.Procs)
	}

	if st.OpenFiles <= 0 {
		t.Errorf("OpenFiles = %d, want > 0", st.OpenFiles)
	}
}
