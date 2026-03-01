// Package pagination provides cursor-based pagination helpers for list endpoints.
package pagination

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// EncodeCursor creates a base64-encoded cursor from an ID and timestamp.
func EncodeCursor(id string, createdAt time.Time) string {
	raw := fmt.Sprintf("%s|%d", id, createdAt.UnixNano())
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a base64-encoded cursor back into an ID and timestamp.
func DecodeCursor(cursor string) (id string, createdAt time.Time, err error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("decode cursor: %w", err)
	}

	parts := strings.SplitN(string(data), "|", 2)
	if len(parts) != 2 {
		return "", time.Time{}, fmt.Errorf("invalid cursor format")
	}

	var nanos int64
	if _, err := fmt.Sscanf(parts[1], "%d", &nanos); err != nil {
		return "", time.Time{}, fmt.Errorf("parse cursor timestamp: %w", err)
	}

	return parts[0], time.Unix(0, nanos), nil
}

// DefaultLimit clamps the requested limit to a sensible range.
// If requested is 0, returns defaultLimit. If requested exceeds maxLimit, returns maxLimit.
func DefaultLimit(requested int32, defaultLimit int32, maxLimit int32) int32 {
	if requested <= 0 {
		return defaultLimit
	}
	if requested > maxLimit {
		return maxLimit
	}
	return requested
}
