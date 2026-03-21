package signals

import "testing"

func TestScoreRelevance(t *testing.T) {
	tests := []struct {
		query, title, snippet string
		wantMin, wantMax      float64
	}{
		{"go channels", "Go channels tutorial", "Learn about Go channels and goroutines", 0.6, 1.0},
		{"go channels", "Python tutorial", "Introduction to Python programming", 0.0, 0.3},
		{"go channels", "Go channels", "", 0.5, 1.0},
		{"", "anything", "anything", 0.0, 0.0},
	}
	for _, tt := range tests {
		score := ScoreRelevance(tt.query, tt.title, tt.snippet)
		if score < tt.wantMin || score > tt.wantMax {
			t.Errorf("ScoreRelevance(%q, %q, %q) = %.2f, want [%.2f, %.2f]",
				tt.query, tt.title, tt.snippet, score, tt.wantMin, tt.wantMax)
		}
	}
}

func TestClassifyLink(t *testing.T) {
	tests := []struct {
		url, text, host, want string
	}{
		{"https://example.com/paper.pdf", "Download PDF", "example.com", "download"},
		{"https://example.com/blog?page=2", "Next page", "example.com", "pagination"},
		{"https://example.com/related", "See also", "example.com", "related"},
		{"https://example.com/home", "home", "example.com", "navigation"},
		{"https://other.com/article", "Interesting article", "example.com", "external"},
	}
	for _, tt := range tests {
		got := ClassifyLink(tt.url, tt.text, tt.host)
		if got != tt.want {
			t.Errorf("ClassifyLink(%q, %q, %q) = %q, want %q", tt.url, tt.text, tt.host, got, tt.want)
		}
	}
}
