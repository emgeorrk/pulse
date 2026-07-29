package updatecheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCheckerCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		current     string
		body        string
		wantVersion string
		wantURL     string
		wantErr     error
		status      int
	}{
		{
			name:        "new release with v prefixes",
			current:     "v1.0.4",
			body:        releaseJSON("v1.1.0", "https://github.com/emgeorrk/pulse/releases/tag/v1.1.0"),
			wantVersion: "v1.1.0",
			wantURL:     "https://github.com/emgeorrk/pulse/releases/tag/v1.1.0",
		},
		{
			name:        "versions without v prefixes",
			current:     "1.0.4",
			body:        releaseJSON("1.1.0", "https://github.com/emgeorrk/pulse/releases/tag/1.1.0"),
			wantVersion: "v1.1.0",
			wantURL:     "https://github.com/emgeorrk/pulse/releases/tag/1.1.0",
		},
		{
			name:    "same version",
			current: "1.0.4",
			body:    releaseJSON("v1.0.4", "https://github.com/emgeorrk/pulse/releases/tag/v1.0.4"),
		},
		{
			name:    "installed version is newer",
			current: "1.1.0",
			body:    releaseJSON("v1.0.4", "https://github.com/emgeorrk/pulse/releases/tag/v1.0.4"),
		},
		{
			name:    "invalid current version",
			current: "development",
			body:    releaseJSON("v1.1.0", "https://github.com/emgeorrk/pulse/releases/tag/v1.1.0"),
			wantErr: errCurrentVersion,
		},
		{
			name:    "invalid release version",
			current: "1.0.4",
			body:    releaseJSON("newest", "https://github.com/emgeorrk/pulse/releases/tag/newest"),
			wantErr: errReleaseVersion,
		},
		{
			name:    "prerelease is rejected defensively",
			current: "1.0.4",
			body:    releaseJSON("v1.1.0-beta.1", "https://github.com/emgeorrk/pulse/releases/tag/v1.1.0-beta.1"),
			wantErr: errReleaseVersion,
		},
		{
			name:    "error status",
			current: "1.0.4",
			status:  http.StatusForbidden,
			body:    `{}`,
			wantErr: errReleaseStatus,
		},
		{
			name:    "malformed json",
			current: "1.0.4",
			body:    `{"tag_name":`,
			wantErr: errReleasePayload,
		},
		{
			name:    "oversized response",
			current: "1.0.4",
			body:    strings.Repeat(" ", maxReleaseBody+1),
			wantErr: errReleaseBody,
		},
		{
			name:    "wrong release host",
			current: "1.0.4",
			body:    releaseJSON("v1.1.0", "https://example.com/emgeorrk/pulse/releases/tag/v1.1.0"),
			wantErr: errReleasePage,
		},
		{
			name:    "release URL tag mismatch",
			current: "1.0.4",
			body:    releaseJSON("v1.1.0", "https://github.com/emgeorrk/pulse/releases/tag/v9.9.9"),
			wantErr: errReleasePage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status := tt.status
			if status == 0 {
				status = http.StatusOK
			}

			checker := New(tt.current)
			checker.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != latestReleaseEndpoint {
					t.Errorf("request URL = %q, want %q", req.URL, latestReleaseEndpoint)
				}

				if req.Header.Get("Accept") != "application/vnd.github+json" {
					t.Errorf("Accept header = %q", req.Header.Get("Accept"))
				}

				if req.Header.Get("X-GitHub-Api-Version") != githubAPIVersion {
					t.Errorf("API version header = %q", req.Header.Get("X-GitHub-Api-Version"))
				}

				return &http.Response{
					StatusCode: status,
					Status:     http.StatusText(status),
					Body:       io.NopCloser(strings.NewReader(tt.body)),
					Header:     make(http.Header),
				}, nil
			})

			got, err := checker.Check(context.Background())
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Check() error = %v, want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("Check() error: %v", err)
			}

			if tt.wantVersion == "" {
				if got != nil {
					t.Errorf("Check() = %+v, want no update", got)
				}

				return
			}

			if got == nil {
				t.Fatal("Check() = nil, want release")
			}

			if got.Version != tt.wantVersion || got.URL != tt.wantURL {
				t.Errorf("Check() = %+v, want version %q URL %q", got, tt.wantVersion, tt.wantURL)
			}
		})
	}
}

func TestCheckerHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	checker := New("1.0.4")
	checker.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()

		return nil, req.Context().Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := checker.Check(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Check() error = %v, want %v", err, context.Canceled)
	}
}

func releaseJSON(tag, page string) string {
	return `{"tag_name":"` + tag + `","html_url":"` + page + `"}`
}
