package fetcher

// The following code was sourced and modified from the
// https://github.com/andrew-d/goscrape package governed by MIT license.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	errs "github.com/jakopako/goskyr/errors"
	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/network"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/protocol/runtime"
	"github.com/mafredri/cdp/rpcc"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ChromeFetcher is used to fetch Java Script rendeded pages.
type ChromeFetcher struct {
	cdpClient *cdp.Client
	client    *http.Client
	cookies   []*http.Cookie
}

// NewChromeFetcher returns ChromeFetcher
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

	return f
}

// Fetch retrieves document from the remote server. It returns web page content along with cache and expiration information.
func (f *ChromeFetcher) Fetch(request Request) (*http.Response, error) {
	//URL validation
	if _, err := url.ParseRequestURI(strings.TrimSpace(request.getURL())); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Context Creation + Devtool creation
	devt := devtool.New(viper.GetString("CHROME"), devtool.WithClient(f.client))
	//https://github.com/mafredri/cdp/issues/60
	//pt, err := devt.Get(ctx, devtool.Page)
	pt, err := devt.Create(ctx)
	if err != nil {
		return nil, err
	}

	//#TODO: Understand what this means, what is CONN? And what does RPCC mean in this case?
	var conn *rpcc.Conn
	if viper.GetBool("CHROME_TRACE") {
		newLogCodec := func(conn io.ReadWriter) rpcc.Codec {
			return &LogCodec{conn: conn}
		}
		// Connect to WebSocket URL (page) that speaks the Chrome Debugging Protocol.
		conn, err = rpcc.DialContext(
			ctx, pt.WebSocketDebuggerURL, rpcc.WithCodec(newLogCodec))
	} else {
		conn, err = rpcc.DialContext(ctx, pt.WebSocketDebuggerURL)
	}
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer conn.Close() // Cleanup.
	defer devt.Close(ctx, pt)

	// Create a new CDP Client that uses conn.
	f.cdpClient = cdp.NewClient(conn)

	if err = runBatch(
		// Enable all the domain events that we're interested in.
		func() error { return f.cdpClient.DOM.Enable(ctx, nil) },
		func() error { return f.cdpClient.Network.Enable(ctx, nil) },
		func() error { return f.cdpClient.Page.Enable(ctx) },
		func() error { return f.cdpClient.Runtime.Enable(ctx) },
	); err != nil {
		return nil, err
	}

	err = f.loadCookies()
	if err != nil {
		return nil, err
	}

	domLoadTimeout := 60 * time.Second
	if request.FormData == "" {
		err = f.navigate(ctx, f.cdpClient.Page, "GET", request.getURL(), "", domLoadTimeout)
	} else {
		formData := parseFormData(request.FormData)
		err = f.navigate(ctx, f.cdpClient.Page, "POST", request.getURL(), formData.Encode(), domLoadTimeout)
	}
	if err != nil {
		return nil, err
	}

	// Q: Huh, we are not killing it here eh? Why not return, instead just warn?
	if err := f.runActions(ctx, request.Actions); err != nil {
		log.Warn(err.Error())
	}

	u, err := url.Parse(request.getURL())
	if err != nil {
		return nil, err
	}
	f.cookies, err = f.saveCookies(u, &ctx)
	if err != nil {
		return nil, err
	}

	// // Fetch the document root node. We can pass nil here
	// // since this method only takes optional arguments.
	// doc, err := f.cdpClient.DOM.GetDocument(ctx, nil)
	// if err != nil {
	// 	return nil, err
	// }

	// // Get the outer HTML for the page.
	// result, err := f.cdpClient.DOM.GetOuterHTML(ctx, &dom.GetOuterHTMLArgs{
	// 	NodeID: &doc.Root.NodeID,
	// })
	// if err != nil {
	// 	return nil, err
	// }
	// readCloser := ioutil.NopCloser(strings.NewReader(result.OuterHTML))

	// #TODO: Understand scrapper better. Do we return *http.response or io.ReaderCloser
	// in fetcher.Fetch?
	return nil, nil

}

func (f *ChromeFetcher) runActions(ctx context.Context, actionsJSON string) error {
	if len(actionsJSON) == 0 {
		return nil
	}
	acts := []map[string]json.RawMessage{}
	err := json.Unmarshal([]byte(actionsJSON), &acts)
	if err != nil {
		return err
	}
	for _, actionMap := range acts {
		for actionType, params := range actionMap {
			action, err := NewAction(actionType, params)
			if err == nil {
				return action.Execute(ctx, f)
			}
		}
	}
	return nil
}

func (f *ChromeFetcher) setCookieJar(jar http.CookieJar) {
	f.client.Jar = jar
}

func (f *ChromeFetcher) getCookieJar() http.CookieJar {
	return f.client.Jar
}

// Static type assertion
var _ Fetcher = &ChromeFetcher{}

// navigate to the URL and wait for DOMContentEventFired. An error is
// returned if timeout happens before DOMContentEventFired.
func (f *ChromeFetcher) navigate(ctx context.Context, pageClient cdp.Page, method, url string, formData string, timeout time.Duration) error {
	defer time.Sleep(750 * time.Millisecond)

	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), timeout)

	// Make sure Page events are enabled.
	err := pageClient.Enable(ctxTimeout)
	if err != nil {
		return err
	}

	// Navigate to GitHub, block until ready.
	loadEventFired, err := pageClient.LoadEventFired(ctxTimeout)
	if err != nil {
		return err
	}
	defer loadEventFired.Close()

	loadingFailed, err := f.cdpClient.Network.LoadingFailed(ctxTimeout)
	if err != nil {
		return err
	}
	defer loadingFailed.Close()

	// exceptionThrown, err := f.cdpClient.Runtime.ExceptionThrown(ctxTimeout)
	// if err != nil {
	// 	return err
	// }
	//defer exceptionThrown.Close()

	if method == "GET" {
		_, err = pageClient.Navigate(ctxTimeout, page.NewNavigateArgs(url))
		if err != nil {
			return err
		}
	} else {
		/* ast := "*" */
		pattern := network.RequestPattern{URLPattern: &url}
		patterns := []network.RequestPattern{pattern}

		f.cdpClient.Network.SetCacheDisabled(ctxTimeout, network.NewSetCacheDisabledArgs(true))

		interArgs := network.NewSetRequestInterceptionArgs(patterns)
		err = f.cdpClient.Network.SetRequestInterception(ctxTimeout, interArgs)
		if err != nil {
			return err
		}

		kill := make(chan bool)
		go f.interceptRequest(ctxTimeout, url, formData, kill)
		_, err = pageClient.Navigate(ctxTimeout, page.NewNavigateArgs(url))
		if err != nil {
			return err
		}
		kill <- true
	}
	select {
	// case <-exceptionThrown.Ready():
	// 	ev, err := exceptionThrown.Recv()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return errs.StatusError{400, errors.New(ev.ExceptionDetails.Error())}
	case <-loadEventFired.Ready():
		_, err = loadEventFired.Recv()
		if err != nil {
			return err
		}
	case <-loadingFailed.Ready():
		reply, err := loadingFailed.Recv()
		if err != nil {
			return err
		}
		canceled := reply.Canceled != nil && *reply.Canceled
		if !canceled && reply.Type == network.ResourceTypeDocument {
			return errs.StatusError{400, errors.New(reply.ErrorText)}
		}
	case <-ctx.Done():
		cancelTimeout()
		return nil /*
			case <-ctxTimeout.Done():
				return errs.StatusError{400, errors.New("Fetch timeout")} */
	}
	return nil
}

func (f *ChromeFetcher) setCookies(u *url.URL, cookies []*http.Cookie) error {
	f.cookies = cookies
	return nil
}

func (f *ChromeFetcher) loadCookies() error {
	/* 	u, err := url.Parse(cookiesURL)
	   	if err != nil {
	   		return err
	   	} */
	for _, c := range f.cookies {
		c1 := network.SetCookieArgs{
			Name:  c.Name,
			Value: c.Value,
			Path:  &c.Path,
			/* Expires:  expire, */
			Domain:   &c.Domain,
			HTTPOnly: &c.HttpOnly,
			Secure:   &c.Secure,
		}
		if !c.Expires.IsZero() {
			duration := c.Expires.Sub(time.Unix(0, 0))
			c1.Expires = network.TimeSinceEpoch(duration / time.Second)
		}
		_, err := f.cdpClient.Network.SetCookie(context.Background(), &c1)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *ChromeFetcher) getCookies(u *url.URL) ([]*http.Cookie, error) {
	return f.cookies, nil
}

func (f *ChromeFetcher) saveCookies(u *url.URL, ctx *context.Context) ([]*http.Cookie, error) {
	ncookies, err := f.cdpClient.Network.GetCookies(*ctx, &network.GetCookiesArgs{URLs: []string{u.String()}})
	if err != nil {
		return nil, err
	}
	cookies := []*http.Cookie{}
	for _, c := range ncookies.Cookies {

		c1 := http.Cookie{
			Name:  c.Name,
			Value: c.Value,
			Path:  c.Path,
			/* Expires:  expire, */
			Domain:   c.Domain,
			HttpOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
		if c.Expires > -1 {
			sec, dec := math.Modf(c.Expires)
			expire := time.Unix(int64(sec), int64(dec*(1e9)))
			/* logger.Info(expire.String())
			log.Info(expire.Format("2006-01-02 15:04:05")) */
			c1.Expires = expire
		}
		cookies = append(cookies, &c1)
		domain := string(c1.Domain)
		Url := u.String()
		f.cdpClient.Network.DeleteCookies(*ctx, &network.DeleteCookiesArgs{Name: c.Name, Domain: &domain, URL: &Url, Path: &c1.Path})
	}
	return cookies, nil
}

func (f *ChromeFetcher) interceptRequest(ctx context.Context, originURL string, formData string, kill chan bool) {
	var sig = false
	cl, err := f.cdpClient.Network.RequestIntercepted(ctx)
	if err != nil {
		panic(err)
	}
	defer cl.Close()
	for {
		if sig {
			return
		}
		select {
		case <-cl.Ready():
			r, err := cl.Recv()
			if err != nil {
				log.Error(err.Error())
				sig = true
				continue
			}

			lengthFormData := len(formData)
			if lengthFormData > 0 && r.Request.URL == originURL && r.RedirectURL == nil {

				interceptedArgs := network.NewContinueInterceptedRequestArgs(r.InterceptionID).
					SetMethod("POST").
					SetPostData(formData)

				headers, _ := json.Marshal(map[string]string{
					"Content-Type":   "application/x-www-form-urlencoded",
					"Content-Length": strconv.Itoa(lengthFormData),
				})
				interceptedArgs.Headers = headers

				if err = f.cdpClient.Network.ContinueInterceptedRequest(ctx, interceptedArgs); err != nil {
					log.Error(err.Error())
					sig = true
					continue
				}
			} else {
				interceptedArgs := network.NewContinueInterceptedRequestArgs(r.InterceptionID)
				if r.ResourceType == network.ResourceTypeImage || r.ResourceType == network.ResourceTypeStylesheet || isExclude(r.Request.URL) {
					interceptedArgs.SetErrorReason(network.ErrorReasonAborted)
				}
				if err = f.cdpClient.Network.ContinueInterceptedRequest(ctx, interceptedArgs); err != nil {
					log.Error(err.Error())
					sig = true
					continue
				}
				continue
			}
		case <-kill:
			sig = true
			break
		}
	}
}

func (f ChromeFetcher) RunJSFromFile(ctx context.Context, path string, entryPointFunction string) error {
	exp, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	exp = append(exp, entryPointFunction...)

	compileReply, err := f.cdpClient.Runtime.CompileScript(ctx, &runtime.CompileScriptArgs{
		Expression:    string(exp),
		PersistScript: true,
	})
	if err != nil {
		panic(err)
	}
	awaitPromise := true

	_, err = f.cdpClient.Runtime.RunScript(ctx, &runtime.RunScriptArgs{
		ScriptID:     *compileReply.ScriptID,
		AwaitPromise: &awaitPromise,
	})
	return err
}
