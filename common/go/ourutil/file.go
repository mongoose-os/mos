// Copyright (c) 2014-2018 Cesanta Software Limited
// All rights reserved

package ourutil

import (
	"io/ioutil"
	"net/http"
	"strings"
	"unicode"

	"github.com/cesanta/errors"
)

func IdentifierFromString(name string) string {
	ret := ""
	for _, c := range name {
		if !(unicode.IsLetter(c) || unicode.IsDigit(c)) {
			c = '_'
		}
		ret += string(c)
	}
	return ret
}
func FileNameFromString(name string) string {
	ret := ""
	for _, c := range name {
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || c == '.' || c == '-') {
			c = '_'
		}
		ret += string(c)
	}
	return ret
}

func ReadOrFetchFile(nameOrURL string) ([]byte, error) {
	if strings.HasPrefix(nameOrURL, "http://") || strings.HasPrefix(nameOrURL, "https://") {
		Reportf("Fetching %s...", nameOrURL)
		resp, err := http.Get(nameOrURL)
		if err != nil {
			return nil, errors.Annotatef(err, "%s: failed to fetch", nameOrURL)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, errors.Errorf("%s: failed to fetch: %s", nameOrURL, resp.Status)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Annotatef(err, "%s: failed to fetch body", nameOrURL)
		}
		Reportf("  done, %d bytes.", len(b))
		return b, nil
	} else {
		return ioutil.ReadFile(nameOrURL)
	}
}
