package post

import (
	"math"
	"time"
)

const (
	// hotGravity controls the time-decay rate for the Hot ranking algorithm.
	// Higher values cause posts to decay faster. Lemmy default is 1.8.
	hotGravity = 1.8

	// hotScaleFactor scales the hot score to produce sort-friendly values.
	hotScaleFactor = 10000.0
)

// HotScore calculates a hot ranking score for a post.
// Uses the Lemmy algorithm: 10000 * log(max(1, 3+score)) / (hoursAge+2)^1.8
// Higher scores mean "hotter" — high votes + recency.
func HotScore(score int, createdAt time.Time) float64 {
	hoursAge := time.Since(createdAt).Hours()
	numerator := hotScaleFactor * math.Log(math.Max(1, float64(3+score)))
	denominator := math.Pow(hoursAge+2, hotGravity)
	return numerator / denominator
}

// RisingScore calculates a rising/velocity ranking score for a post.
// Rising = score / max(1, hoursAge) — posts gaining votes quickly rank higher.
// Only meaningful for recent posts (last 24h).
func RisingScore(score int, createdAt time.Time) float64 {
	hoursAge := time.Since(createdAt).Hours()
	return float64(score) / math.Max(1, hoursAge)
}
