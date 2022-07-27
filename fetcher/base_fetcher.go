package fetcher

// The following code was sourced and modified from the
// https://github.com/andrew-d/goscrape package governed by MIT license.

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	ourErrors "github.com/jakopako/goskyr/errors"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/publicsuffix"
)

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Code int
	Err  error
}

// BaseFetcher is a Fetcher that uses the Go standard library's http
// client to fetch URLs.
type BaseFetcher struct {
	client *http.Client
}

// newBaseFetcher creates instances of newBaseFetcher{} to fetch
// a page content from regular websites as-is
// without running js scripts on the page.
func newBaseFetcher() *BaseFetcher {
	var client *http.Client
	proxy := viper.GetString("PROXY")
	if len(proxy) > 0 {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			log.Error(err.Error())
			return nil
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		client = &http.Client{Transport: transport}
	} else {
		client = &http.Client{}
	}
	f := &BaseFetcher{
		client: client,
	}
	jarOpts := &cookiejar.Options{PublicSuffixList: publicsuffix.List}
	var err error
	f.client.Jar, err = cookiejar.New(jarOpts)
	if err != nil {
		return nil
	}
	return f
}

// Fetch retrieves document from the remote server.
func (bf *BaseFetcher) Fetch(request Request) (*http.Response, error) {
	resp, err := bf.response(request)
	if err != nil {
		return nil, err
	}
	// NOT Converting fetched content to UTF-8
	// utf8Res, _, _, err := readerToUtf8Encoding(resp.Body)
	// if err != nil {
	// 	return nil, err
	// }
	return resp, err
}

//Response return response after document fetching using BaseFetcher
func (bf *BaseFetcher) response(r Request) (*http.Response, error) {
	//URL validation
	if _, err := url.ParseRequestURI(r.getURL()); err != nil {
		return nil, err
	}
	var err error
	var req *http.Request

	if r.FormData == "" {
		req, err = http.NewRequest(r.Method, r.URL, nil)
		if err != nil {
			return nil, err
		}
	} else {
		//if form data exists send POST request
		formData := parseFormData(r.FormData)
		req, err = http.NewRequest("POST", r.URL, strings.NewReader(formData.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Content-Length", strconv.Itoa(len(formData.Encode())))
	}
	//TODO: Add UA to requests
	//req.Header.Add("User-Agent", "Dataflow kit - https://github.com/slotix/dataflowkit")
	return bf.doRequest(req)
}

func (bf *BaseFetcher) doRequest(req *http.Request) (*http.Response, error) {
	resp, err := bf.client.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case 200:
		return resp, nil

	default:
		return nil, ourErrors.StatusError{
			resp.StatusCode,
			errors.New(http.StatusText(resp.StatusCode)),
		}
	}
}

func (bf *BaseFetcher) getCookieJar() http.CookieJar { //*cookiejar.Jar {
	return bf.client.Jar
}

func (bf *BaseFetcher) setCookieJar(jar http.CookieJar) {

	bf.client.Jar = jar
}

func (bf *BaseFetcher) getCookies(u *url.URL) ([]*http.Cookie, error) {
	return bf.client.Jar.Cookies(u), nil
}

func (bf *BaseFetcher) setCookies(u *url.URL, cookies []*http.Cookie) error {
	bf.client.Jar.SetCookies(u, cookies)
	return nil
}
