package extractors

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/chromedp/chromedp"
)

const BrowserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
const idleTimeout = 3 * time.Minute

var (
	allocCtx    context.Context
	allocCancel context.CancelFunc
	allocMu     sync.Mutex
	idleTimer   *time.Timer
)

var tagStripper = regexp.MustCompile(`<[^>]+>`)

func stripTags(html string) string {
	return tagStripper.ReplaceAllString(html, "")
}

func isSPA(html string) bool {
	lower := strings.ToLower(html)
	if !strings.Contains(lower, "<script") || !strings.Contains(lower, "<body") {
		return false
	}
	return len(strings.TrimSpace(stripTags(html))) < 300
}

func isAntibot(html string, status int) bool {
	if status == 403 || status == 429 || status == 503 {
		return true
	}
	lower := strings.ToLower(html)
	for _, m := range []string{
		"just a moment", "cf-browser-verification", "enable javascript and cookies",
		"captcha", "hcaptcha", "recaptcha", "please verify you are a human",
		"datadome", "_incapsula_resource", "access denied", "automated access", "unusual traffic",
	} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

func addBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", BrowserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
}

// HTTPFetch performs a plain HTTP GET and returns body string + status code.
func HTTPFetch(ctx context.Context, rawURL string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", 0, err
	}
	addBrowserHeaders(req)
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(body), resp.StatusCode, nil
}

// HTTPFetchBytes performs a plain HTTP GET and returns raw bytes + Content-Type header.
func HTTPFetchBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", BrowserUA)
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.Header.Get("Content-Type"), err
}

func getOrStartAllocator() context.Context {
	allocMu.Lock()
	defer allocMu.Unlock()
	if allocCancel == nil {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.UserAgent(BrowserUA),
		)
		allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	}
	if idleTimer != nil {
		idleTimer.Reset(idleTimeout)
	} else {
		idleTimer = time.AfterFunc(idleTimeout, func() {
			allocMu.Lock()
			defer allocMu.Unlock()
			if allocCancel != nil {
				allocCancel()
				allocCancel = nil
				idleTimer = nil
			}
		})
	}
	return allocCtx
}

// BrowserFetch renders a URL using headless Chrome and returns the outer HTML.
func BrowserFetch(rawURL string) (string, error) {
	ctx, cancel := chromedp.NewContext(getOrStartAllocator())
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(rawURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Poll(`document.body.innerText.length > 200`, nil,
			chromedp.WithPollingTimeout(8*time.Second),
			chromedp.WithPollingInterval(500*time.Millisecond),
		),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	return html, err
}

// FetchHTML fetches a URL as HTML, using browser fallback for SPAs and anti-bot pages.
func FetchHTML(ctx context.Context, rawURL string) (string, error) {
	html, status, err := HTTPFetch(ctx, rawURL)
	if err != nil {
		return "", err
	}
	if isSPA(html) || isAntibot(html, status) {
		return BrowserFetch(rawURL)
	}
	return html, nil
}

var mdConverter = md.NewConverter("", true, nil)

// HTMLToMarkdown converts HTML to clean, readable markdown.
func HTMLToMarkdown(htmlStr string) string {
	result, err := mdConverter.ConvertString(htmlStr)
	if err != nil {
		return strings.TrimSpace(stripTags(htmlStr))
	}
	return result
}
