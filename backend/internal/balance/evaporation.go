package balance

import (
	"fmt"
	"math"
	"time"
)

type EvaporationEstimate struct {
	PeriodDays    float64 `json:"period_days"`
	EstimatedKG   float64 `json:"estimated_kg"`
	UncertaintyKG float64 `json:"uncertainty_kg"`
}

// EstimateEvaporation derives the normal boil-off mass over the balance period
// from the tank daily evaporation rate (% of inventory per day) and propagates
// the rate uncertainty into an absolute mass uncertainty.
func EstimateEvaporation(inventoryMassKG, dailyRatePct, rateUncertaintyPct float64, period time.Duration) (EvaporationEstimate, error) {
	if inventoryMassKG < 0 || !finite(inventoryMassKG) {
		return EvaporationEstimate{}, fmt.Errorf("inventory mass %.3f kg is invalid", inventoryMassKG)
	}
	if err := ValidateBOGRate(dailyRatePct, rateUncertaintyPct); err != nil {
		return EvaporationEstimate{}, err
	}
	if period <= 0 {
		return EvaporationEstimate{}, fmt.Errorf("balance period must be positive to estimate evaporation")
	}
	days := period.Hours() / 24
	estimated := inventoryMassKG * PercentFraction(dailyRatePct) * days
	if !finite(estimated) || estimated < 0 {
		return EvaporationEstimate{}, fmt.Errorf("estimated evaporation mass is invalid")
	}
	uncertainty := estimated * PercentFraction(rateUncertaintyPct)
	if !finite(uncertainty) || uncertainty < 0 {
		return EvaporationEstimate{}, fmt.Errorf("evaporation uncertainty is invalid")
	}
	return EvaporationEstimate{
		PeriodDays:    Round(days, 6),
		EstimatedKG:   Round(estimated, 3),
		UncertaintyKG: Round(uncertainty, 3),
	}, nil
}

// CombineAbsoluteUncertainty folds absolute uncertainty components (kg) into a
// single root-sum-of-squares combined uncertainty.
func CombineAbsoluteUncertainty(components ...float64) (float64, error) {
	if len(components) == 0 {
		return 0, fmt.Errorf("at least one absolute uncertainty component is required")
	}
	sumSquares := 0.0
	for index, component := range components {
		if component < 0 || !finite(component) {
			return 0, fmt.Errorf("absolute uncertainty component %d is invalid", index)
		}
		sumSquares += component * component
	}
	return Round(math.Sqrt(sumSquares), 3), nil
}
