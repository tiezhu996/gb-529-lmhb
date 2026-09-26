package service

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/datatypes"

	"lng-boiloff-gas-balance/backend/internal/balance"
	"lng-boiloff-gas-balance/backend/internal/constants"
	"lng-boiloff-gas-balance/backend/internal/model"
)

func TestCalculateBalanceRunProducesReplayEvidence(t *testing.T) {
	curve, _ := balance.NewCapacityCurve([]float64{0, 15000})
	raw, _ := curve.Marshal()
	tank := model.StorageTank{
		ID: 1, TankCode: "TK-TEST", NominalCapacityM3: 180000, MinLevelM: 0, MaxLevelM: 12,
		ReferenceDensityKGM3: 452, ReferenceTemperatureC: -160, ThermalExpansionPerC: 0.0035,
		BOGRateKGPerDay: 45000, BOGRateUncertaintyPct: 5,
		CapacityCurveJSON: datatypes.JSON(raw), CoefficientVersion: "CV-T1", TankStatus: "active",
	}
	start := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	opening := model.MeasurementSnapshot{
		ID: 10, TankID: 1, MeasuredAt: start.Add(-time.Hour), CalculatedLiquidMassKG: 55_000_000,
		MeasurementUncertaintyPct: 0.30, QualityFlag: constants.QualityGood,
	}
	closing := model.MeasurementSnapshot{
		ID: 11, TankID: 1, MeasuredAt: end.Add(-time.Hour), CalculatedLiquidMassKG: 55_100_000,
		MeasurementUncertaintyPct: 0.32, QualityFlag: constants.QualityGood,
	}
	transfers := []model.TransferOperation{
		{ID: 20, OperationType: "inflow", MeasuredMassKG: 250000, MeasurementUncertaintyPct: 0.2},
		{ID: 21, OperationType: "outflow", MeasuredMassKG: 90000, MeasurementUncertaintyPct: 0.25},
	}
	calculated, snapshot, evidence, err := calculateBalanceRun(tank, opening, closing, transfers, start, end)
	if err != nil {
		t.Fatalf("calculate run: %v", err)
	}
	if calculated.NetTransferKG != 160000 {
		t.Fatalf("unexpected net transfer: %+v", calculated)
	}
	if calculated.EstimatedBOGKG != 45000 || calculated.UnexplainedKG != 15000 {
		t.Fatalf("total deviation 60000 must split into normal BOG 45000 and unexplained 15000: %+v", calculated)
	}
	if calculated.InputAnomaly {
		t.Fatal("normal BOG below total deviation must not flag an input anomaly")
	}
	if len(snapshot) < 200 || len(evidence) < 200 {
		t.Fatalf("expected replay snapshot and evidence, got %d/%d bytes", len(snapshot), len(evidence))
	}
	if calculated.DeviationLevel != constants.DeviationWithinUncertainty {
		t.Fatalf("unexpected deviation level: %s", calculated.DeviationLevel)
	}
}

func TestCalculateBalanceRunFlagsInputAnomaly(t *testing.T) {
	curve, _ := balance.NewCapacityCurve([]float64{0, 15000})
	raw, _ := curve.Marshal()
	tank := model.StorageTank{
		ID: 1, TankCode: "TK-TEST", NominalCapacityM3: 180000, MinLevelM: 0, MaxLevelM: 12,
		ReferenceDensityKGM3: 452, ReferenceTemperatureC: -160, ThermalExpansionPerC: 0.0035,
		BOGRateKGPerDay: 80000, BOGRateUncertaintyPct: 5,
		CapacityCurveJSON: datatypes.JSON(raw), CoefficientVersion: "CV-T1", TankStatus: "active",
	}
	start := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	opening := model.MeasurementSnapshot{
		ID: 10, TankID: 1, MeasuredAt: start.Add(-time.Hour), CalculatedLiquidMassKG: 55_000_000,
		MeasurementUncertaintyPct: 0.30, QualityFlag: constants.QualityGood,
	}
	closing := model.MeasurementSnapshot{
		ID: 11, TankID: 1, MeasuredAt: end.Add(-time.Hour), CalculatedLiquidMassKG: 55_100_000,
		MeasurementUncertaintyPct: 0.32, QualityFlag: constants.QualityGood,
	}
	transfers := []model.TransferOperation{
		{ID: 20, OperationType: "inflow", MeasuredMassKG: 250000, MeasurementUncertaintyPct: 0.2},
		{ID: 21, OperationType: "outflow", MeasuredMassKG: 90000, MeasurementUncertaintyPct: 0.25},
	}
	calculated, _, evidence, err := calculateBalanceRun(tank, opening, closing, transfers, start, end)
	if err != nil {
		t.Fatalf("calculate run: %v", err)
	}
	if !calculated.InputAnomaly {
		t.Fatal("normal BOG 80000 exceeding total deviation 60000 must flag an input anomaly")
	}
	if calculated.UnexplainedKG != -20000 {
		t.Fatalf("unexpected unexplained remainder: %+v", calculated)
	}
	if calculated.DeviationLevel != constants.DeviationInvalid {
		t.Fatalf("anomalous input must classify as invalid, got %s", calculated.DeviationLevel)
	}
	var decoded struct {
		InputAnomaly  bool   `json:"input_anomaly"`
		AnomalyReason string `json:"anomaly_reason"`
	}
	if err := json.Unmarshal(evidence, &decoded); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if !decoded.InputAnomaly || decoded.AnomalyReason == "" {
		t.Fatalf("evidence must record the input anomaly, got %+v", decoded)
	}
}

func TestBalanceStateMachine(t *testing.T) {
	valid := []struct {
		from constants.BalanceStatus
		to   constants.BalanceStatus
	}{
		{constants.BalanceQueued, constants.BalanceCalculating},
		{constants.BalanceCalculating, constants.BalancePendingReview},
		{constants.BalancePendingReview, constants.BalanceAccepted},
		{constants.BalancePendingReview, constants.BalanceRejected},
	}
	for _, transition := range valid {
		if !constants.CanTransitionBalance(transition.from, transition.to) {
			t.Fatalf("expected valid transition %s -> %s", transition.from, transition.to)
		}
	}
	if constants.CanTransitionBalance(constants.BalanceAccepted, constants.BalanceCalculating) {
		t.Fatal("accepted result must remain immutable")
	}
	if constants.CanTransitionBalance(constants.BalanceInputAnomaly, constants.BalancePendingReview) {
		t.Fatal("input anomaly runs must not be submitted for review")
	}
	if !constants.CanTransitionBalance(constants.BalanceInputAnomaly, constants.BalanceInvalidated) {
		t.Fatal("input anomaly runs must remain invalidatable")
	}
}
