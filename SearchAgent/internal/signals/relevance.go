package signals

import (
	"math"
	"strings"
	"unicode"
)

// ScoreRelevance returns 0.0–1.0 indicating how relevant title+snippet are to query.
// Title matches are weighted 2× snippet matches.
func ScoreRelevance(query, title, snippet string) float64 {
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return 0
	}
	titleScore := termOverlap(queryTerms, tokenize(title)) * 2.0
	snippetScore := termOverlap(queryTerms, tokenize(snippet))
	return math.Min((titleScore+snippetScore)/3.0, 1.0)
}

// termOverlap returns fraction of query terms present in target.
func termOverlap(query, target []string) float64 {
	if len(query) == 0 {
		return 0
	}
	targetSet := make(map[string]bool, len(target))
	for _, t := range target {
		targetSet[t] = true
	}
	hits := 0
	for _, q := range query {
		if targetSet[q] {
			hits++
		}
	}
	return float64(hits) / float64(len(query))
}

// tokenize lowercases and splits s into word tokens.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	var cur strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}
