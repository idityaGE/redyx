// Package spam implements spam detection with keyword/URL blocklists,
// content deduplication, and behavior analysis via Kafka consumer.
package spam

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// blocklistData represents the JSON structure of the blocklist seed file.
type blocklistData struct {
	Keywords []string `json:"keywords"`
	Domains  []string `json:"domains"`
}

// Blocklist provides in-memory keyword and URL domain checking.
// Keywords are stored lowercased for case-insensitive matching.
// Domains are stored in a map for O(1) lookup.
type Blocklist struct {
	keywords []string
	domains  map[string]bool
}

// LoadBlocklist reads a JSON blocklist file and returns a Blocklist.
// Keywords are normalized to lowercase during loading.
func LoadBlocklist(path string) (*Blocklist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read blocklist file: %w", err)
	}

	var raw blocklistData
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse blocklist json: %w", err)
	}

	// Normalize keywords to lowercase for case-insensitive matching
	keywords := make([]string, len(raw.Keywords))
	for i, kw := range raw.Keywords {
		keywords[i] = strings.ToLower(kw)
	}

	// Build domain lookup map
	domains := make(map[string]bool, len(raw.Domains))
	for _, d := range raw.Domains {
		domains[strings.ToLower(d)] = true
	}

	return &Blocklist{
		keywords: keywords,
		domains:  domains,
	}, nil
}

// CheckKeywords performs case-insensitive substring matching against the keyword list.
// Returns (matched, keyword) where keyword is the matched keyword (for internal logging only).
func (b *Blocklist) CheckKeywords(content string) (bool, string) {
	lower := strings.ToLower(content)
	for _, kw := range b.keywords {
		if strings.Contains(lower, kw) {
			return true, kw
		}
	}
	return false, ""
}

// CheckURLs checks each URL against the blocked domains list.
// Parses each URL to extract the hostname, then checks against the domain map.
// Returns (matched, domain) where domain is the matched domain.
func (b *Blocklist) CheckURLs(urls []string) (bool, string) {
	for _, rawURL := range urls {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if b.domains[host] {
			return true, host
		}
	}
	return false, ""
}

// urlRegex matches bare URLs (http:// or https://).
var urlRegex = regexp.MustCompile(`https?://[^\s\)]+`)

// markdownLinkRegex matches markdown-style links: [text](url).
var markdownLinkRegex = regexp.MustCompile(`\[.*?\]\((https?://[^\s\)]+)\)`)

// ExtractURLs finds all URLs in content, including bare URLs and markdown links.
// Markdown link URLs are extracted from the parenthesized portion.
func ExtractURLs(content string) []string {
	seen := make(map[string]bool)
	var result []string

	// Extract markdown link URLs first (more specific pattern)
	for _, match := range markdownLinkRegex.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			u := match[1]
			if !seen[u] {
				seen[u] = true
				result = append(result, u)
			}
		}
	}

	// Extract bare URLs
	for _, u := range urlRegex.FindAllString(content, -1) {
		if !seen[u] {
			seen[u] = true
			result = append(result, u)
		}
	}

	return result
}
