// Package fetch is the one place that talks HTTP. Each registry package
// builds its URLs and decodes the bodies; this package only moves bytes.
package fetch

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// Response is what a GET came back with. Next is the URL of the following
// page when the server sent a Link header with rel="next", else empty.
// RateLimitRemaining is the X-RateLimit-Remaining header as sent, else
// empty; GitHub uses it to say that a 403 is the hourly limit.
type Response struct {
	Status             int
	Body               []byte
	Next               string
	RateLimitRemaining string
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
		return Response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("reading %s: %w", url, err)
	}
	return Response{
		Status:             resp.StatusCode,
		Body:               body,
		Next:               NextLink(resp.Header.Get("Link")),
		RateLimitRemaining: resp.Header.Get("X-RateLimit-Remaining"),
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
