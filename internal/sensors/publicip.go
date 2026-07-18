package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/emgeorrk/pulse/internal/entity"
)

const (
	// publicIPTimeout bounds one provider request; Fetch tries each provider
	// at most once.
	publicIPTimeout = 10 * time.Second
	// publicIPMaxBody caps the response read: the largest reply (myip.com
	// JSON) is well under a hundred bytes.
	publicIPMaxBody = 256
	// countryUnknown is how MyIP.com marks an unresolved country; not a
	// real ISO code.
	countryUnknown = "XX"
	// countryCodeLen is the ISO 3166-1 alpha-2 length.
	countryCodeLen = 2
)

// ipProvider is one lookup endpoint plus the decoder for its response
// format. Geolocating providers fill Country; the plain-text ones leave it
// empty, so the IP itself keeps working when the geo services are down.
type ipProvider struct {
	parse func(body []byte) (entity.PublicIPInfo, error)
	url   string
}

// PublicIP looks up the machine's public IP address — and, when the provider
// reports it, the country — over HTTPS. It is not a hardware sensor: it is
// only queried when the user enables the metric, on a slow schedule owned by
// the usecase layer.
type PublicIP struct {
	client    *http.Client
	providers []ipProvider
}

func NewPublicIP() *PublicIP {
	return &PublicIP{
		client: &http.Client{Timeout: publicIPTimeout},
		providers: []ipProvider{
			{url: "https://api.myip.com", parse: parseMyIP},
			{url: "https://api.country.is", parse: parseCountryIs},
			{url: "https://api.ipify.org", parse: parsePlainIP},
			{url: "https://checkip.amazonaws.com", parse: parsePlainIP},
		},
	}
}

// Fetch returns the result from the first provider that answers with a
// valid address.
func (p *PublicIP) Fetch(ctx context.Context) (entity.PublicIPInfo, error) {
	var lastErr error

	for _, prov := range p.providers {
		info, err := p.fetchOne(ctx, prov)
		if err == nil {
			return info, nil
		}

		lastErr = err
	}

	return entity.PublicIPInfo{}, lastErr
}

func (p *PublicIP) fetchOne(ctx context.Context, prov ipProvider) (entity.PublicIPInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, prov.url, http.NoBody)
	if err != nil {
		return entity.PublicIPInfo{}, fmt.Errorf("public IP request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return entity.PublicIPInfo{}, fmt.Errorf("public IP fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return entity.PublicIPInfo{}, fmt.Errorf("%w: %s from %s", errPublicIPStatus, resp.Status, prov.url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, publicIPMaxBody))
	if err != nil {
		return entity.PublicIPInfo{}, fmt.Errorf("public IP body: %w", err)
	}

	info, err := prov.parse(body)
	if err != nil {
		return entity.PublicIPInfo{}, fmt.Errorf("%w from %s", err, prov.url)
	}

	return info, nil
}

// parseMyIP decodes api.myip.com: {"ip":"…","country":"Netherlands","cc":"NL"}.
func parseMyIP(body []byte) (entity.PublicIPInfo, error) {
	var v struct {
		IP string `json:"ip"`
		CC string `json:"cc"`
	}

	if err := json.Unmarshal(body, &v); err != nil {
		return entity.PublicIPInfo{}, fmt.Errorf("%w: %q", errPublicIPInvalid, body)
	}

	return newIPInfo(v.IP, v.CC)
}

// parseCountryIs decodes api.country.is: {"ip":"…","country":"NL"}.
func parseCountryIs(body []byte) (entity.PublicIPInfo, error) {
	var v struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
	}

	if err := json.Unmarshal(body, &v); err != nil {
		return entity.PublicIPInfo{}, fmt.Errorf("%w: %q", errPublicIPInvalid, body)
	}

	return newIPInfo(v.IP, v.Country)
}

// parsePlainIP handles the bare-text providers (ipify, checkip): the IP
// only, no country.
func parsePlainIP(body []byte) (entity.PublicIPInfo, error) {
	return newIPInfo(string(body), "")
}

// newIPInfo validates the address and normalizes the country code. A bad
// country never fails the lookup — the IP alone is a valid result.
func newIPInfo(ip, country string) (entity.PublicIPInfo, error) {
	ip = strings.TrimSpace(ip)
	if net.ParseIP(ip) == nil {
		return entity.PublicIPInfo{}, fmt.Errorf("%w: %q", errPublicIPInvalid, ip)
	}

	return entity.PublicIPInfo{IP: ip, Country: countryCode(country)}, nil
}

// countryCode returns the upper-cased ISO 3166-1 alpha-2 code, or "" for
// anything else (MyIP.com reports an unknown country as "XX").
func countryCode(cc string) string {
	cc = strings.ToUpper(strings.TrimSpace(cc))
	if len(cc) != countryCodeLen || cc == countryUnknown {
		return ""
	}

	for _, c := range cc {
		if c < 'A' || c > 'Z' {
			return ""
		}
	}

	return cc
}
