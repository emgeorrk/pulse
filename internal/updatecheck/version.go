// Package updatecheck discovers the installed Pulse version and checks
// GitHub for a newer stable release.
package updatecheck

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	bundleContentsDir = "Contents"
	bundleMacOSDir    = "MacOS"
	bundlePlistName   = "Info.plist"
	bundleVersionKey  = "CFBundleShortVersionString"
	maxBundlePlist    = 1024 * 1024
)

// CurrentVersion reads CFBundleShortVersionString from the .app bundle that
// contains the running executable. Resolving symlinks makes this work for the
// command symlink installed by the Homebrew formula as well as a directly
// launched Pulse.app.
func CurrentVersion() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}

	return currentVersionFromExecutable(executable)
}

func currentVersionFromExecutable(executable string) (string, error) {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve app executable: %w", err)
	}

	macosDir, contentsDir := filepath.Dir(resolved), filepath.Dir(filepath.Dir(resolved))
	if filepath.Base(macosDir) != bundleMacOSDir || filepath.Base(contentsDir) != bundleContentsDir {
		return "", fmt.Errorf("%w: %s", errBundlePath, resolved)
	}

	plistPath := filepath.Join(contentsDir, bundlePlistName)

	plist, err := os.Open(plistPath)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errBundlePlist, err)
	}
	defer plist.Close()

	limited := io.LimitReader(plist, maxBundlePlist+1)

	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errBundlePlist, err)
	}

	if len(data) > maxBundlePlist {
		return "", fmt.Errorf("%w: larger than %d bytes", errBundlePlist, maxBundlePlist)
	}

	return versionFromPlist(data)
}

func versionFromPlist(data []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errBundleVersion
			}

			return "", fmt.Errorf("%w: %w", errBundlePlist, err)
		}

		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "key" {
			continue
		}

		var key string
		if err := decoder.DecodeElement(&key, &start); err != nil {
			return "", fmt.Errorf("%w: %w", errBundlePlist, err)
		}

		if key != bundleVersionKey {
			continue
		}

		return plistString(decoder)
	}
}

func plistString(decoder *xml.Decoder) (string, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("%w: %w", errBundlePlist, err)
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		if start.Name.Local != "string" {
			return "", fmt.Errorf("%w: %s is not a string", errBundleVersion, bundleVersionKey)
		}

		var version string
		if err := decoder.DecodeElement(&version, &start); err != nil {
			return "", fmt.Errorf("%w: %w", errBundlePlist, err)
		}

		version = strings.TrimSpace(version)
		if version == "" {
			return "", errBundleVersion
		}

		return version, nil
	}
}
