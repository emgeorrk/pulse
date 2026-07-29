package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	latestReleaseEndpoint = "https://api.github.com/repos/emgeorrk/pulse/releases/latest"
	releasePageHost       = "github.com"
	releasePagePath       = "/emgeorrk/pulse/releases/tag/"
	githubAPIVersion      = "2022-11-28"
	releaseRequestTimeout = 10 * time.Second
	maxReleaseBody        = 32 * 1024
)

// Release identifies a newer stable Pulse release and its GitHub page.
type Release struct {
	Version string
	URL     string
}

type releasePayload struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Source is the update-checking dependency consumed by the tray UI.
type Source interface {
	Check(ctx context.Context) (*Release, error)
}

// Checker compares the installed app version with GitHub's latest stable
// release. It owns only short-lived requests; scheduling belongs to the UI.
type Checker struct {
	client         *http.Client
	currentVersion string
}

func New(currentVersion string) *Checker {
	return &Checker{
		client:         &http.Client{Timeout: releaseRequestTimeout},
		currentVersion: currentVersion,
	}
}

// Check returns nil when the latest stable release is not newer than the
// installed app. Failures are returned so callers can preserve their existing
// UI state rather than mistaking a network error for "up to date".
func (c *Checker) Check(ctx context.Context) (*Release, error) {
	current := normalizedVersion(c.currentVersion)
	if !semver.IsValid(current) {
		return nil, fmt.Errorf("%w: %q", errCurrentVersion, c.currentVersion)
	}

	payload, err := c.latestRelease(ctx, current)
	if err != nil {
		return nil, err
	}

	latest := normalizedVersion(payload.TagName)
	if !semver.IsValid(latest) || semver.Prerelease(latest) != "" {
		return nil, fmt.Errorf("%w: %q", errReleaseVersion, payload.TagName)
	}

	if semver.Compare(latest, current) <= 0 {
		return nil, nil
	}

	releaseURL, err := validatedReleaseURL(payload.HTMLURL, strings.TrimSpace(payload.TagName))
	if err != nil {
		return nil, err
	}

	return &Release{Version: latest, URL: releaseURL}, nil
}

func (c *Checker) latestRelease(ctx context.Context, current string) (releasePayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseEndpoint, http.NoBody)
	if err != nil {
		return releasePayload{}, fmt.Errorf("create GitHub release request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	req.Header.Set("User-Agent", "Pulse/"+strings.TrimPrefix(current, "v"))

	resp, err := c.client.Do(req)
	if err != nil {
		return releasePayload{}, fmt.Errorf("fetch latest GitHub release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return releasePayload{}, fmt.Errorf("%w: %s", errReleaseStatus, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseBody+1))
	if err != nil {
		return releasePayload{}, fmt.Errorf("read GitHub release response: %w", err)
	}

	if len(body) > maxReleaseBody {
		return releasePayload{}, fmt.Errorf("%w: larger than %d bytes", errReleaseBody, maxReleaseBody)
	}

	var payload releasePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return releasePayload{}, fmt.Errorf("%w: %w", errReleasePayload, err)
	}

	return payload, nil
}

func normalizedVersion(version string) string {
	version = strings.TrimSpace(version)
	if strings.HasPrefix(version, "v") {
		return version
	}

	return "v" + version
}

func validatedReleaseURL(rawURL, tag string) (string, error) {
	releaseURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errReleasePage, err)
	}

	valid := releaseURL.Scheme == "https" &&
		releaseURL.Host == releasePageHost &&
		releaseURL.User == nil &&
		releaseURL.RawQuery == "" &&
		releaseURL.Fragment == "" &&
		releaseURL.Path == releasePagePath+tag
	if !valid {
		return "", fmt.Errorf("%w: %q", errReleasePage, rawURL)
	}

	return releaseURL.String(), nil
}
