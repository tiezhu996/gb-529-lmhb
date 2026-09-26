package balance

import (
	"fmt"
	"time"
)

// EstimateNormalBOG converts the tank daily boil-off rate into an expected
// normal evaporation mass over the balance period.
func EstimateNormalBOG(rateKGPerDay float64, period time.Duration) (float64, error) {
	if err := ValidateBOGRate(rateKGPerDay); err != nil {
		return 0, err
	}
	if period <= 0 {
		return 0, fmt.Errorf("balance period must be positive to estimate normal boil-off")
	}
	estimated := rateKGPerDay * (period.Hours() / 24)
	if !finite(estimated) || estimated < 0 {
		return 0, fmt.Errorf("normal boil-off estimate is invalid")
	}
	return Round(estimated, 3), nil
}

// UnexplainedDeviation deducts the normal boil-off estimate from the total
// physical deviation; the remainder is the unexplained part.
func UnexplainedDeviation(totalDeviation, normalBOG float64) (float64, error) {
	if !finite(totalDeviation) || !finite(normalBOG) || normalBOG < 0 {
		return 0, fmt.Errorf("deviation split requires finite inputs and non-negative normal boil-off")
	}
	return Round(totalDeviation-normalBOG, 3), nil
}

// IsInputAnomaly reports whether the estimated normal evaporation exceeds the
// total deviation, which indicates boundary snapshots or transfer metering
// are inconsistent rather than a real gain of mass.
func IsInputAnomaly(normalBOG, totalDeviation float64) bool {
	if !finite(normalBOG) || !finite(totalDeviation) {
		return true
	}
	return normalBOG > totalDeviation
}
