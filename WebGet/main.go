package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
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
	hasScript := strings.Contains(lower, "<script")
	hasBody := strings.Contains(lower, "<body")
	if !hasScript || !hasBody {
		return false
	}
	text := strings.TrimSpace(stripTags(html))
	return len(text) < 300
}

func isAntibot(html string, status int) bool {
	if status == 403 || status == 429 || status == 503 {
		return true
	}
	lower := strings.ToLower(html)
	markers := []string{
		"just a moment",
		"cf-browser-verification",
		"enable javascript and cookies",
		"captcha",
		"hcaptcha",
		"recaptcha",
		"please verify you are a human",
		"datadome",
		"_incapsula_resource",
		"access denied",
		"automated access",
		"unusual traffic",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func httpFetch(ctx context.Context, url string) (string, int, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
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

func getOrStartAllocator() context.Context {
	allocMu.Lock()
	defer allocMu.Unlock()

	if allocCancel == nil {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.UserAgent(browserUA),
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

func browserFetch(url string) (string, error) {
	ctx, cancel := chromedp.NewContext(getOrStartAllocator())
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Poll(`document.body.innerText.length > 200`, nil,
			chromedp.WithPollingTimeout(8*time.Second),
			chromedp.WithPollingInterval(500*time.Millisecond),
		),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	return html, err
}

type WebGetInput struct {
	URL string `json:"url"`
}

func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}

func toolText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func getWebpageHTML(ctx context.Context, req *mcp.CallToolRequest, args WebGetInput) (*mcp.CallToolResult, any, error) {
	html, status, err := httpFetch(ctx, args.URL)
	if err != nil {
		return toolError(fmt.Sprintf("HTTP fetch failed: %v", err)), nil, nil
	}
	if isSPA(html) || isAntibot(html, status) {
		html, err = browserFetch(args.URL)
		if err != nil {
			return toolError(fmt.Sprintf("Browser render failed: %v", err)), nil, nil
		}
	}
	return toolText(html), nil, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "webget", Version: "v0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_webpage_html",
		Description: "Fetch a webpage's HTML. Automatically handles JS-rendered (SPA) pages and anti-bot protections using headless Chrome. Chrome is kept warm between calls.",
	}, getWebpageHTML)
	if err := server.Run(context.Background(), mcp.NewStdioTransport()); err != nil {
		log.Fatal(err)
	}
}
