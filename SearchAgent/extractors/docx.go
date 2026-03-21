package extractors

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ExtractDOCX extracts plain text from .docx bytes using the embedded word/document.xml.
func ExtractDOCX(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("opening docx: %w", err)
	}
	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("reading document.xml: %w", err)
		}
		defer rc.Close()
		xmlBytes, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return parseDocxXML(xmlBytes), nil
	}
	return "", fmt.Errorf("word/document.xml not found in docx")
}

// parseDocxXML extracts text from a Word document.xml byte slice.
func parseDocxXML(xmlData []byte) string {
	var sb strings.Builder
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	inText := false
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t": // w:t — text run
				inText = true
			case "p": // w:p — paragraph break
				sb.WriteString("\n")
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}
