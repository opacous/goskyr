package utils

import (
	"net/http"

	"github.com/jakopako/goskyr/fetcher"
)

type Request = fetcher.Request

// type Fetcher interface {
// 	func FetchURL() (io.ReadCloser, error)
// }

// ()

func FetchUrl(url string, userAgent string) (*http.Response, error) {
	// NOTE: body has to be closed by caller
	// 2022.07.26 12:28 PM - Let's just get it to use baseFetcher for now.
	fetcher := fetcher.NewFetcher("Base")

	// 1. Make Response from URL + params
	request := Request{
		Type:      "Base",
		URL:       url,
		Method:    "GET",
		UserToken: userAgent,
		Actions:   "",
	}

	// 2. Make said request and get the readerCloser
	response, err := fetcher.Fetch(request)
	if err != nil {
		return nil, err
	}

	return response, err
}
