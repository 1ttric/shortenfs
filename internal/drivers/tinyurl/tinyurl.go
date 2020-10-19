// Tinyurl is pleasant because write operations with the same data always result in the same shortlink, and because
// there is no rate limiting.
package tinyurl

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/1ttric/shortenfs/internal/drivers"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var (
	reFindID   = regexp.MustCompile("https://preview.tinyurl.com/([a-zA-Z0-9]+)")
	httpClient = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
)

func init() {
	drivers.Register("tinyurl", &Tinyurl{})
}

// With overhead, maximum storable bytes per link is 8135 (as of 10/14/20)
type Tinyurl struct{}

func (t Tinyurl) NodeSize() int {
	return 6096
}

func (t Tinyurl) IdSize() int {
	return 8
}

func (t Tinyurl) Write(data []byte) (string, error) {
	readUrl, err := url.Parse("https://tinyurl.com/create.php")
	if err != nil {
		panic(err)
	}
	// There is no domain validation to work around!
	encodedUrl := "http://" + base64.RawURLEncoding.EncodeToString(data)
	q := readUrl.Query()
	q.Set("source", "index")
	q.Set("url", encodedUrl)
	q.Set("alias", "")
	readUrl.RawQuery = q.Encode()
	urlStr := readUrl.String()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", errors.Wrap(err, "could not build request")
	}
	req.Header.Add("Host", "tinyurl.com")
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; GNU/Linux) AppleWebKit/537.36 (KHTML, like Gecko) Chromium/79.0.3945.130 Chrome/79.0.3945.130 Safari/537.36 Tesla/2020.16.2.1-e99c70fff409")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "could not perform request")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "could not read response")
	}

	match := reFindID.FindStringSubmatch(string(body))
	if len(match) < 2 {
		return "", fmt.Errorf("no id found")
	}
	id := match[1]

	return id, nil
}

func (t Tinyurl) Read(id string) ([]byte, error) {
	readUrl := "https://tinyurl.com/" + id

	req, err := http.NewRequest("GET", readUrl, nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not build request")
	}
	req.Header.Add("Host", "tinyurl.com")
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; GNU/Linux) AppleWebKit/537.36 (KHTML, like Gecko) Chromium/79.0.3945.130 Chrome/79.0.3945.130 Safari/537.36 Tesla/2020.16.2.1-e99c70fff409")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "could not perform request")
	}

	header := resp.Header.Get("Location")

	if header == "" {
		return nil, fmt.Errorf("response is not a redirect")
	}
	encodedData := header
	if !strings.HasPrefix(encodedData, "http://") {
		return nil, fmt.Errorf("redirect URL is of unexpected format")
	}
	encodedData = strings.TrimPrefix(encodedData, "http://")
	data, err := base64.RawURLEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, fmt.Errorf("could not decode base64 data")
	}

	return data, nil
}
