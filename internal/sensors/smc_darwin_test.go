//go:build darwin

package sensors

import "testing"

// intelTemperatureKeys is a static curated list, so a single invariant check:
// it must be non-empty and free of duplicate SMC keys (a duplicate would
// double-count a sensor in the Temps output).
func TestIntelTemperatureKeys(t *testing.T) {
	t.Parallel()

	keys := intelTemperatureKeys()
	if len(keys) == 0 {
		t.Fatal("intelTemperatureKeys() is empty")
	}

	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		if k.key == "" || k.label == "" {
			t.Errorf("entry has an empty field: %+v", k)
		}

		if seen[k.key] {
			t.Errorf("duplicate SMC key %q", k.key)
		}

		seen[k.key] = true
	}
}
