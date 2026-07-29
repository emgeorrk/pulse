package updatecheck

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentVersionFromExecutable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		plist   string
		want    string
		wantErr error
		symlink bool
	}{
		{
			name:  "bundle version",
			plist: testPlist("1.2.3"),
			want:  "1.2.3",
		},
		{
			name:    "homebrew command symlink",
			plist:   testPlist("2.0.1"),
			want:    "2.0.1",
			symlink: true,
		},
		{
			name:    "missing plist",
			wantErr: errBundlePlist,
		},
		{
			name:    "malformed plist",
			plist:   "<plist><dict><key>CFBundleShortVersionString</key>",
			wantErr: errBundlePlist,
		},
		{
			name:    "missing version key",
			plist:   testPlistWithKey("CFBundleName", "Pulse"),
			wantErr: errBundleVersion,
		},
		{
			name:    "version has wrong value type",
			plist:   `<?xml version="1.0"?><plist><dict><key>CFBundleShortVersionString</key><integer>1</integer></dict></plist>`,
			wantErr: errBundleVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			executable := filepath.Join(root, "Pulse.app", bundleContentsDir, bundleMacOSDir, "pulse")

			if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(executable, nil, 0o755); err != nil {
				t.Fatal(err)
			}

			if tt.plist != "" {
				plistPath := filepath.Join(root, "Pulse.app", bundleContentsDir, bundlePlistName)
				if err := os.WriteFile(plistPath, []byte(tt.plist), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			path := executable
			if tt.symlink {
				binDir := filepath.Join(root, "bin")
				if err := os.MkdirAll(binDir, 0o755); err != nil {
					t.Fatal(err)
				}

				path = filepath.Join(binDir, "pulse")
				if err := os.Symlink(executable, path); err != nil {
					t.Fatal(err)
				}
			}

			got, err := currentVersionFromExecutable(path)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("currentVersionFromExecutable() error = %v, want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("currentVersionFromExecutable() error: %v", err)
			}

			if got != tt.want {
				t.Errorf("currentVersionFromExecutable() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentVersionRejectsNonBundleExecutable(t *testing.T) {
	t.Parallel()

	executable := filepath.Join(t.TempDir(), "pulse")
	if err := os.WriteFile(executable, nil, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := currentVersionFromExecutable(executable); !errors.Is(err, errBundlePath) {
		t.Errorf("currentVersionFromExecutable() error = %v, want %v", err, errBundlePath)
	}
}

func testPlist(version string) string {
	return testPlistWithKey(bundleVersionKey, version)
}

func testPlistWithKey(key, value string) string {
	return `<?xml version="1.0"?>
<plist version="1.0">
<dict>
	<key>` + key + `</key>
	<string>` + value + `</string>
</dict>
</plist>`
}
