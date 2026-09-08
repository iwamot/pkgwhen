package npm

import (
	"fmt"

	"github.com/iwamot/pkgwhen/internal/fetch"
	"github.com/iwamot/pkgwhen/internal/release"
)

// List fetches every version of name. found is false when the registry has
// no such package.
func List(name string) (rs []release.Release, found bool, err error) {
	resp, err := fetch.Get(URL(name), nil)
	if err != nil {
		return nil, false, err
	}
	switch resp.Status {
	case 200:
		rs, err = Decode(resp.Body)
		return rs, true, err
	case 404:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("HTTP %d", resp.Status)
	}
}
