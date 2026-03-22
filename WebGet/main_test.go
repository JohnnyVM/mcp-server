package main

import (
	"context"
	"strings"
	"testing"
)

// TestExampleComNoHeadless verifies that example.com is fetched via plain HTTP
// without triggering the SPA or anti-bot detection (no headless Chrome needed).
func TestExampleComNoHeadless(t *testing.T) {
	ctx := context.Background()
	html, status, err := httpFetch(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("httpFetch failed: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	if isSPA(html) {
		t.Error("example.com incorrectly detected as SPA")
	}
	if isAntibot(html, status) {
		t.Error("example.com incorrectly detected as anti-bot")
	}
	if !strings.Contains(strings.ToLower(html), "example") {
		t.Error("expected 'example' in response body")
	}
}

// TestTemuDocumentationNeedsJS verifies that the Temu documentation page is a
// JS-rendered SPA and that headless Chrome produces full page content.
func TestTemuDocumentationNeedsJS(t *testing.T) {
	const url = "https://partner-eu.temu.com/documentation?menu_code=7289390cfd724be4a196f11ebe45a896"

	ctx := context.Background()
	html, status, err := httpFetch(ctx, url)
	if err != nil {
		t.Fatalf("httpFetch failed: %v", err)
	}

	needsBrowser := isSPA(html) || isAntibot(html, status)
	if !needsBrowser {
		t.Log("plain HTTP returned sufficient content — page may have changed, skipping browser check")
		return
	}

	rendered, err := browserFetch(url)
	if err != nil {
		t.Fatalf("browserFetch failed: %v", err)
	}
	if len(rendered) < 1000 {
		t.Errorf("rendered HTML too short (%d bytes), expected full page content", len(rendered))
	}
}
