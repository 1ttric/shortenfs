// Note that bitly implements rate limiting that WILL kick in shortly after you start sending too many requests.
package bitly

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/1ttric/shortenfs/internal/drivers"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"strings"
)

var (
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
	drivers.Register("bitly", &Bitly{})
}

type apiResponse struct {
	StatusCode int `json:"status_code"`
	Data       struct {
		Archived   bool          `json:"archived"`
		Tags       []interface{} `json:"tags"`
		CreatedAt  string        `json:"created_at"`
		Deeplinks  []interface{} `json:"deeplinks"`
		LongURL    string        `json:"long_url"`
		References struct {
			Group string `json:"group"`
		} `json:"references"`
		CustomBitlinks []interface{} `json:"custom_bitlinks"`
		Link           string        `json:"link"`
		ID             string        `json:"id"`
	} `json:"data"`
	StatusTxt string `json:"status_txt"`
}

type Bitly struct{}

func (b Bitly) NodeSize() int {
	return 1527
}

func (b Bitly) IdSize() int {
	return 7
}

func (b Bitly) Write(data []byte) (string, error) {
	dataB64 := base64.RawURLEncoding.EncodeToString(data)
	// Bitly validates domains per RFC1035, and against a list of real TLDs - so the data is put in the path instead
	encodedUrl := "http://_.co/" + dataB64
	req, err := http.NewRequest("POST", "https://bitly.com/data/anon_shorten", strings.NewReader("url="+encodedUrl))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Host", "bitly.com")
	req.Header.Add("X-XSRFToken", "ffffffffffffffffffffffffffffffff")
	req.Header.Add("Cookie", "_xsrf=ffffffffffffffffffffffffffffffff")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Linux; U; Android 4.0.4; en-us; Glass 1 Build/IMM76L; XE12) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "could not perform request")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "could not read response")
	}

	var j apiResponse
	err = json.Unmarshal(body, &j)
	if err != nil {
		fmt.Println(string(body))
		return "", errors.Wrap(err, "could not read response json")
	}

	if j.Data.ID == "" {
		return "", fmt.Errorf("api response code %d (%s)", j.StatusCode, j.StatusTxt)
	}
	if !strings.HasPrefix(j.Data.ID, "bit.ly/") {
		return "", fmt.Errorf("expected prefix not found")
	}
	id := strings.TrimPrefix(j.Data.ID, "bit.ly/")
	return id, nil
}

func (b Bitly) Read(id string) ([]byte, error) {
	readUrl := "https://bit.ly/" + id

	req, err := http.NewRequest("GET", readUrl, nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not build request")
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Linux; U; Android 4.0.4; en-us; Glass 1 Build/IMM76L; XE12) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "could not perform request")
	}

	header := resp.Header.Get("Location")

	if header == "" {
		return nil, fmt.Errorf("response is not a redirect")
	}
	encodedData := header
	if !strings.HasPrefix(encodedData, "http://_.co/") {
		return nil, fmt.Errorf("redirect URL is of unexpected format")
	}
	encodedData = strings.TrimPrefix(encodedData, "http://_.co/")

	data, err := base64.RawURLEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode base64 data")
	}
	return data, nil
}
