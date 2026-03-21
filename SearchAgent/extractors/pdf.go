package extractors

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractPDF extracts plain text from PDF bytes, with page markers.
func ExtractPDF(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("opening PDF: %w", err)
	}
	var sb strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		if i > 1 {
			fmt.Fprintf(&sb, "\n\n--- Page %d ---\n\n", i)
		}
		sb.WriteString(text)
	}
	return strings.TrimSpace(sb.String()), nil
}
