// Package comment implements the CommentService gRPC server with ScyllaDB
// storage, materialized path tree ordering, Wilson score ranking, and
// Kafka-based vote score updates.
package comment

import (
	"fmt"
	"strings"
)

// NextPath generates the next child path segment for a parent.
// counter is the incremented counter value from ScyllaDB counter table.
// parentPath is "" for top-level comments, "001" for children of comment "001", etc.
// Returns full path: e.g., "001" (top-level) or "001.002" (reply to "001").
func NextPath(parentPath string, counter int64) string {
	segment := fmt.Sprintf("%03d", counter)
	if parentPath == "" {
		return segment
	}
	return parentPath + "." + segment
}

// ParentPath returns the parent's path from a child path.
// "001.002.003" -> "001.002", "001" -> "" (top-level).
func ParentPath(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx == -1 {
		return ""
	}
	return path[:idx]
}

// Depth returns the depth of a path (number of segments).
// "001" -> 1, "001.002" -> 2, "001.002.003" -> 3.
func Depth(path string) int {
	if path == "" {
		return 0
	}
	return strings.Count(path, ".") + 1
}

// IsDescendant checks if childPath is a descendant of ancestorPath.
func IsDescendant(childPath, ancestorPath string) bool {
	return strings.HasPrefix(childPath, ancestorPath+".")
}
