package utils

import "math"

// PartialExposureA8 returns the partial A(8) vibration exposure value (m/s²)
// for a given vibration magnitude (m/s²) and trigger time (minutes).
func PartialExposureA8(vibrationMagnitude float64, triggerTime int) float64 {
	return vibrationMagnitude * math.Sqrt((float64(triggerTime)/60.0)/8.0)
}

// PartialExposurePoints returns the rounded partial exposure points
// for a given vibration magnitude (m/s²) and trigger time (minutes).
func PartialExposurePoints(vibrationMagnitude float64, triggerTime int) float64 {
	points := math.Pow((vibrationMagnitude/2.5), 2) * ((float64(triggerTime) / 60.0) / 8.0 * 100)
	return math.Round(points)
}
