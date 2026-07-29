package updatecheck

import "errors"

var (
	errBundlePath     = errors.New("executable is not inside an app bundle")
	errBundlePlist    = errors.New("app bundle Info.plist is unreadable")
	errBundleVersion  = errors.New("app bundle version is unavailable")
	errCurrentVersion = errors.New("current app version is invalid")
	errReleaseStatus  = errors.New("GitHub release endpoint returned an error status")
	errReleaseBody    = errors.New("GitHub release response is too large")
	errReleasePayload = errors.New("GitHub release response is invalid")
	errReleaseVersion = errors.New("GitHub release version is invalid")
	errReleasePage    = errors.New("GitHub release page URL is invalid")
)
