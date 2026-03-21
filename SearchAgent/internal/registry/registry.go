package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Source defines a named research source and how to query it.
type Source struct {
	Name         string            `json:"name"`
	Label        string            `json:"label"`
	Category     string            `json:"category"`
	QueryURL     string            `json:"query_url"`
	ResultType   string            `json:"result_type"`   // "json-api" | "html-links" | "html-article"
	ResultFields map[string]string `json:"result_fields"` // field mapping for json-api type
	ContentTypes []string          `json:"content_types"`
	Description  string            `json:"description"`
}

// Registry holds all known sources.
type Registry struct {
	sources []Source
}

// Load reads and parses a sources.json file.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading sources from %s: %w", path, err)
	}
	var sources []Source
	if err := json.Unmarshal(data, &sources); err != nil {
		return nil, fmt.Errorf("parsing sources: %w", err)
	}
	return &Registry{sources: sources}, nil
}

// DefaultPath returns the sources.json path: $SOURCES_FILE env, or next to the binary.
func DefaultPath() string {
	if p := os.Getenv("SOURCES_FILE"); p != "" {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return "sources.json"
	}
	return filepath.Join(filepath.Dir(exe), "sources.json")
}

// All returns all sources.
func (r *Registry) All() []Source { return r.sources }

// ByCategory returns sources matching the given category.
func (r *Registry) ByCategory(category string) []Source {
	var out []Source
	for _, s := range r.sources {
		if s.Category == category {
			out = append(out, s)
		}
	}
	return out
}

// ByName returns a source by name, or false if not found.
func (r *Registry) ByName(name string) (Source, bool) {
	for _, s := range r.sources {
		if s.Name == name {
			return s, true
		}
	}
	return Source{}, false
}
