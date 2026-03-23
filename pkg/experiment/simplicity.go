package experiment

// CalculateSimplicity computes a simplicity score for a code change.
// The score reflects improvement per unit of code complexity introduced.
//
// Positive score: metric improved relative to baseline.
// Negative score: metric got worse.
// Zero: no metric change.
//
// Direction controls which way is "better":
//   - Minimize: lower current is better (improvement = baseline - current)
//   - Maximize: higher current is better (improvement = current - baseline)
//
// Lines complexity factor: max(1, linesAdded + linesRemoved) prevents
// division by zero and penalizes larger changes.
//
// When baseline is zero the raw delta is returned with no ratio normalization.
func CalculateSimplicity(baseline, current float64, linesAdded, linesRemoved int, dir Direction) float64 {
	var delta float64
	switch dir {
	case Minimize:
		delta = baseline - current
	case Maximize:
		delta = current - baseline
	default:
		delta = baseline - current
	}

	if delta == 0 {
		return 0.0
	}

	// @AX:NOTE [AUTO]: When baseline is zero, ratio normalization is skipped to avoid division by zero.
	// Raw delta is returned instead, preserving sign (positive = improvement).
	var improvementRatio float64
	if baseline == 0 {
		improvementRatio = delta
	} else {
		improvementRatio = delta / baseline
	}

	totalLines := linesAdded + linesRemoved
	complexityFactor := float64(totalLines)
	if complexityFactor < 1 {
		complexityFactor = 1
	}

	return improvementRatio / complexityFactor
}
