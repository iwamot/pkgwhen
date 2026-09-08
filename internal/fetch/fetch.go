// Package fetch is the one place that talks HTTP. Each registry package
// builds its URLs and decodes the bodies; this package only moves bytes.
package fetch

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iwamot/pkgwhen/internal/release"
)

// Response is what a GET came back with. Next is the URL of the following
// page when the server sent a Link header with rel="next", else empty. The
// three rate-limit fields are the headers as sent, else empty: GitHub uses
// RateLimitRemaining to say that a 403 is a limit, and answers with either
// RateLimitReset (an epoch second) or RetryAfter (seconds to wait).
type Response struct {
	Status             int
	Body               []byte
	Next               string
	RateLimitRemaining string
	RateLimitReset     string
	RetryAfter         string
}

var client = &http.Client{Timeout: 60 * time.Second}

// Get performs one GET. Gzip is negotiated by net/http on its own, which
// matters for npm packuments that run to megabytes. A status other than
// 200 is returned, not treated as an error, so callers can tell 404 apart.
func Get(url string, headers map[string]string) (Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("User-Agent", "pkgwhen (https://github.com/iwamot/pkgwhen)")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("%w; check the network, then retry", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("reading %s: %w; check the network, then retry", url, err)
	}
	return Response{
		Status:             resp.StatusCode,
		Body:               body,
		Next:               NextLink(resp.Header.Get("Link")),
		RateLimitRemaining: resp.Header.Get("X-RateLimit-Remaining"),
		RateLimitReset:     resp.Header.Get("X-RateLimit-Reset"),
		RetryAfter:         resp.Header.Get("Retry-After"),
	}, nil
}

var nextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// NextLink pulls the rel="next" URL out of a Link header, or returns "".
func NextLink(header string) string {
	m := nextLink.FindStringSubmatch(header)
	if m == nil {
		return ""
	}
	return m[1]
}

// StatusError words a response that is neither the document nor a 404, so
// that the line says what to do next: a 429 is a rate limit and carries the
// wait, a 5xx is the registry failing and worth another call, and anything
// else is the registry refusing this request, where a second try is the
// most that helps. Registries with a token of their own wrap this to speak
// about the token first.
func StatusError(resp Response, now time.Time) error {
	if resp.Status == http.StatusTooManyRequests {
		return errors.New(RateLimited(resp, now))
	}
	if resp.Status >= 500 {
		return fmt.Errorf("HTTP %d; the registry is failing, retry later", resp.Status)
	}
	return fmt.Errorf("unexpected HTTP %d; retry once, and if it persists the registry is rejecting the request", resp.Status)
}

// RateLimited says how long to wait. GitHub sends Retry-After for the
// short-term limit and X-RateLimit-Reset for the hourly one, so whichever
// arrived decides the wording; with neither, or with a reset that has
// already passed, the docs say to wait a minute. The wait is relative
// because the rest of the output is, and a caller reading it may not know
// the current time.
func RateLimited(resp Response, now time.Time) string {
	if s, err := strconv.Atoi(strings.TrimSpace(resp.RetryAfter)); err == nil && s > 0 {
		return fmt.Sprintf("rate limited; retry after %ds", s)
	}
	if resp.RateLimitRemaining == "0" {
		if sec, err := strconv.ParseInt(strings.TrimSpace(resp.RateLimitReset), 10, 64); err == nil {
			if reset := time.Unix(sec, 0).UTC(); reset.After(now) {
				return fmt.Sprintf("rate limited for %s", release.Age(reset, now))
			}
		}
	}
	return "rate limited; wait at least a minute"
}
