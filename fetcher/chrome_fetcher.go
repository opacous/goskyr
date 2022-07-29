package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	neturl "net/url"
	"strings"
	"time"

	// "github.com/PuerkitoBio/goquery"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/publicsuffix"
)

// type Fetcher interface {
// 	Fetch(request Request) (*http.Response, error)
// 	// #TODO: export cookies
// 	// #TODO: import cookies
// }

type ChromeFetcher struct {
	ctx    *context.Context
	client *http.Client
}

func newChromeFetcher() *ChromeFetcher {
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
	f := &ChromeFetcher{
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

func NewCdpContext(timeout int) (context.Context, context.CancelFunc) {
	// Use a executive allocator to use chrome with head
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false))...)

	ctx, cancel = chromedp.NewContext(
		ctx,
		// chromedp.WithDebugf(log.Printf),
	)

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)

	return ctx, cancel
}

// You give me Selector string & place to save it, I give u the corresponding action
// func actionGrabHTMLToSelection(selector string, placeToSave *[]*cdp.Node) *chromedp.Tasks {
// 	return &chromedp.Tasks{
// 		chromedp.WaitVisible(selector),
// 		chromedp.Nodes(selector, placeToSave),
// 	}
// }

func (cf *ChromeFetcher) Fetch(request Request) (*http.Response, error) {
	chromeContext, cancelContext := NewCdpContext(40)
	defer cancelContext()

	var response string
	var statusCode int64
	var responseHeaders map[string]interface{}
	url := request.URL

	runError := chromedp.Run(
		chromeContext,
		chromeTask(
			chromeContext, url,
			request.Header,
			&response, &statusCode, &responseHeaders))

	if runError != nil {
		panic(runError)
	}

	// fmt.Printf(
	// 	"\n\n{%s}\n\n > %s\n status: %d\nheaders: %s\n\n",
	// 	response, url, statusCode, responseHeaders,
	// )

	// we have the responseBody, status code, and response headers now!
	composedResponse := http.Response{}
	composedResponse.Body = io.NopCloser(strings.NewReader(response))
	composedResponse.Header = request.Header
	composedResponse.Status = fmt.Sprint(statusCode)
	composedURL, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}
	composedResponseRequest := http.Request{
		URL: composedURL,
	}
	composedResponse.Request = &composedResponseRequest

	return &composedResponse, nil
}

func chromeTask(chromeContext context.Context, url string, requestHeaders http.Header, response *string, statusCode *int64, responseHeaders *map[string]interface{}) chromedp.Tasks {
	chromedp.ListenTarget(chromeContext, func(event interface{}) {
		switch responseReceivedEvent := event.(type) {
		case *network.EventResponseReceived:
			response := responseReceivedEvent.Response
			if response.URL == url {
				*statusCode = response.Status
				*responseHeaders = response.Headers
			}
		}
	})

	requestHeaderBase := map[string][]string(requestHeaders)
	newRequestHeader := make(map[string]interface{})
	for key, el := range requestHeaderBase {
		newRequestHeader[key] = el
	}

	getResponseBody :=
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			*response, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)

			return err
		})

	return chromedp.Tasks{
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(newRequestHeader)),
		chromedp.Navigate(url),
		getResponseBody,
	}
}

func (cf *ChromeFetcher) getCookieJar() http.CookieJar                        { return nil }
func (cf *ChromeFetcher) setCookieJar(jar http.CookieJar)                     {}
func (cf *ChromeFetcher) getCookies(u *url.URL) ([]*http.Cookie, error)       { return nil, nil }
func (cf *ChromeFetcher) setCookies(u *url.URL, cookies []*http.Cookie) error { return nil }

// func (cf *ChromeFetcher) Fetch(request Request) (*http.Response, error) {
// 	ctx, cancel := NewCdpContext(40)
// 	cf.ctx = &ctx
// 	defer cancel()
// 	var nodes []*cdp.Node

// 	// Timer Start
// 	start := time.Now()

// 	// Bucket of responses
// 	responses := []*http.Response{}

// 	listenForNetworkEvent(*cf.ctx, &responses)

// 	err := chromedp.Run(*cf.ctx,
// 		network.Enable(),
// 		chromedp.Navigate(`https://www.austintexas.gov/cityclerk/boards_commissions/meetings/2017_15_1.htm`),
// 		// wait for footer element is visible (ie, page is loaded)
// 		// chromedp.WaitVisible(`.bcic_doc , b`),
// 		// // Take the table HTML and save it somewhere
// 		// actionGrabHTMLToSelection(`.bcic_doc , b`, &nodes),
// 	)
// 	t := time.Now()
// 	fmt.Println("t at: ", t)

// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	end := time.Now()
// 	fmt.Println("Ending at: ", end)
// 	for _, el := range nodes {
// 		fmt.Println(el.Dump("", "  ", false))
// 	}

// 	fmt.Printf("\nTook: %f secs\n", time.Since(start).Seconds())

// }

// func listenForNetworkEvent(ctx context.Context, responses *[]*http.Response) *http.Response {
// 	chromedp.ListenTarget(ctx, func(ev interface{}) {
// 		switch ev := ev.(type) {

// 		case *network.EventResponseReceived:
// 			resp := ev.Response
// 			if len(resp.Headers) != 0 {
// 				log.Printf("received headers: %s", resp.Headers)
// 				responses = append(responses, &resp)
// 			}

// 		}
// 		case *network.EventResponseReceived:
// 			resp := ev.Response
// 			if len(resp.Headers) != 0 {
// 				log.Printf("received headers: %s", resp.Headers)
// 				responses = append(responses, &resp)
// 			}

// 		}

// 		// other needed network Event
// 	})
// }
