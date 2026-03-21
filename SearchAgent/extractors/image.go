package extractors

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ImageResult holds a fetched image's raw bytes and MIME type.
type ImageResult struct {
	Data     []byte
	MIMEType string
}

const maxImageSize = 10 * 1024 * 1024 // 10 MB

// FetchImage downloads an image and returns it as base64-encoded data.
func FetchImage(ctx context.Context, rawURL string) (*ImageResult, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", BrowserUA)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching image: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize))
	if err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" || !strings.HasPrefix(mimeType, "image/") {
		mimeType = "image/jpeg"
	}
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	return &ImageResult{
		Data:     data,
		MIMEType: mimeType,
	}, nil
}
