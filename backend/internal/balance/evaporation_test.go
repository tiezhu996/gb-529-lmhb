package balance

import (
	"testing"
	"time"
)

func TestEstimateEvaporationScalesWithPeriod(t *testing.T) {
	estimate, err := EstimateEvaporation(55_000_000, 0.05, 20, 48*time.Hour)
	if err != nil {
		t.Fatalf("estimate evaporation: %v", err)
	}
	if estimate.PeriodDays != 2 {
		t.Fatalf("unexpected period days: %.6f", estimate.PeriodDays)
	}
	if estimate.EstimatedKG != 55000 {
		t.Fatalf("unexpected estimated evaporation: %.3f", estimate.EstimatedKG)
	}
	if estimate.UncertaintyKG != 11000 {
		t.Fatalf("unexpected evaporation uncertainty: %.3f", estimate.UncertaintyKG)
	}
}

func TestEstimateEvaporationRejectsInvalidInput(t *testing.T) {
	if _, err := EstimateEvaporation(1000, 0.05, 0, 24*time.Hour); err == nil {
		t.Fatal("positive boil-off rate without rate uncertainty must fail")
	}
	if _, err := EstimateEvaporation(1000, 6, 20, 24*time.Hour); err == nil {
		t.Fatal("boil-off rate above engineering boundary must fail")
	}
	if _, err := EstimateEvaporation(1000, 0.05, 20, 0); err == nil {
		t.Fatal("non-positive balance period must fail")
	}
	if _, err := EstimateEvaporation(-1, 0.05, 20, 24*time.Hour); err == nil {
		t.Fatal("negative inventory mass must fail")
	}
	zero, err := EstimateEvaporation(1000, 0, 0, 24*time.Hour)
	if err != nil {
		t.Fatalf("zero boil-off rate must be allowed: %v", err)
	}
	if zero.EstimatedKG != 0 || zero.UncertaintyKG != 0 {
		t.Fatalf("zero rate must produce zero estimate: %+v", zero)
	}
}

func TestCombineAbsoluteUncertainty(t *testing.T) {
	combined, err := CombineAbsoluteUncertainty(300, 400)
	if err != nil {
		t.Fatalf("combine absolute uncertainty: %v", err)
	}
	if combined != 500 {
		t.Fatalf("unexpected combined uncertainty: %.3f", combined)
	}
	if _, err := CombineAbsoluteUncertainty(); err == nil {
		t.Fatal("empty component list must fail")
	}
	if _, err := CombineAbsoluteUncertainty(300, -1); err == nil {
		t.Fatal("negative component must fail")
	}
}
