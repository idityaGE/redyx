// Package notification implements the notification-service backend with
// Kafka consumer, PostgreSQL storage, WebSocket hub, and mention detection.
package notification

import (
	"regexp"
	"strings"
)

// mentionRe matches u/username patterns in comment bodies.
// Usernames are 3-20 alphanumeric characters plus underscores.
var mentionRe = regexp.MustCompile(`(?:^|\s)u/([a-zA-Z0-9_]{3,20})`)

// ExtractMentions returns a deduplicated list of usernames mentioned in body
// via the u/username pattern. The u/ prefix is stripped from returned names.
func ExtractMentions(body string) []string {
	matches := mentionRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	var usernames []string

	for _, m := range matches {
		username := strings.ToLower(m[1])
		if _, ok := seen[username]; ok {
			continue
		}
		seen[username] = struct{}{}
		usernames = append(usernames, username)
	}

	return usernames
}
