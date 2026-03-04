package comment

import "math"

// WilsonScore computes the lower bound of the Wilson score confidence interval.
// Used for "Best" comment sorting — surfaces quality comments by upvote ratio
// while accounting for sample size.
// z = 1.96 for 95% confidence interval (same as Reddit's original algorithm).
// Returns 0.0 for comments with no votes (sort to bottom).
func WilsonScore(upvotes, downvotes int) float64 {
	n := float64(upvotes + downvotes)
	if n == 0 {
		return 0
	}
	const z = 1.96
	phat := float64(upvotes) / n
	z2 := z * z
	numerator := phat + z2/(2*n) - z*math.Sqrt((phat*(1-phat)+z2/(4*n))/n)
	denominator := 1 + z2/n
	return numerator / denominator
}
