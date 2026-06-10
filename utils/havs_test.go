package utils_test

import (
	"math"
	"testing"

	"github.com/ctrl-hub/challenge/utils"
)

func TestPartialExposureA8(t *testing.T) {
	cases := []struct {
		name               string
		vibrationMagnitude float64
		triggerTime        int
		expected           float64
	}{
		{
			name:               "AirCat Drill at 60 minutes (1 hour at full A8 weighting)",
			vibrationMagnitude: 2.1,
			triggerTime:        60,
			expected:           2.1 * math.Sqrt(1.0/8.0),
		},
		{
			name:               "JCB Hydraulic Breaker at 30 minutes",
			vibrationMagnitude: 4.0,
			triggerTime:        30,
			expected:           4.0 * math.Sqrt(0.5 / 8.0),
		},
		{
			name:               "zero duration yields zero A8",
			vibrationMagnitude: 2.1,
			triggerTime:        0,
			expected:           0,
		},
		{
			name:               "sub-hour duration does not truncate to zero",
			vibrationMagnitude: 4.0,
			triggerTime:        15,
			expected:           4.0 * math.Sqrt((15.0/60.0)/8.0),
		},
		{
			name:               "8-hour workday at EAV vibration level reaches EAV exactly",
			vibrationMagnitude: 2.5,
			triggerTime:        480,
			expected:           2.5, // A(8) = 2.5 * sqrt(8/8) = 2.5
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := utils.PartialExposureA8(tc.vibrationMagnitude, tc.triggerTime)
			if math.Abs(got-tc.expected) > 0.0001 {
				t.Errorf("PartialExposureA8(%v, %d): expected %v, got %v",
					tc.vibrationMagnitude, tc.triggerTime, tc.expected, got)
			}
		})
	}
}

func TestPartialExposurePoints(t *testing.T) {
	cases := []struct {
		name               string
		vibrationMagnitude float64
		triggerTime        int
		expected           float64
	}{
		{
			name:               "8-hour workday at EAV vibration level yields 100 points",
			vibrationMagnitude: 2.5,
			triggerTime:        480,
			expected:           100,
		},
		{
			name:               "8-hour workday at ELV vibration level yields 400 points",
			vibrationMagnitude: 5.0,
			triggerTime:        480,
			expected:           400,
		},
		{
			name:               "zero duration yields zero points",
			vibrationMagnitude: 4.0,
			triggerTime:        0,
			expected:           0,
		},
		{
			name:               "sub-hour duration does not truncate to zero",
			vibrationMagnitude: 4.0,
			triggerTime:        15,
			// (4/2.5)^2 * ((15/60)/8) * 100 = 2.56 * 0.03125 * 100 = 8
			expected: 8,
		},
		{
			name:               "JCB Breaker at 60 minutes yields correct points",
			vibrationMagnitude: 4.0,
			triggerTime:        60,
			// (4/2.5)^2 * (1/8) * 100 = 2.56 * 0.125 * 100 = 32
			expected: 32,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := utils.PartialExposurePoints(tc.vibrationMagnitude, tc.triggerTime)
			if got != tc.expected {
				t.Errorf("PartialExposurePoints(%v, %d): expected %v, got %v",
					tc.vibrationMagnitude, tc.triggerTime, tc.expected, got)
			}
		})
	}
}
