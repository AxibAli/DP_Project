package rules

import (
	"testing"

	"livestock/internal/models"
)

func TestEvaluate_Normal(t *testing.T) {
	e := Evaluate("cow", models.Reading{Temperature: 38.7, HeartRate: 60})
	if e.Abnormal {
		t.Fatalf("expected normal reading, got abnormal: %+v", e)
	}
}

func TestEvaluate_Warning_TempOnly(t *testing.T) {
	e := Evaluate("cow", models.Reading{Temperature: 40.5, HeartRate: 60})
	if !e.Abnormal || e.Severity != SeverityWarning {
		t.Fatalf("expected warning, got %+v", e)
	}
}

func TestEvaluate_Warning_HROnly(t *testing.T) {
	e := Evaluate("cow", models.Reading{Temperature: 38.7, HeartRate: 120})
	if !e.Abnormal || e.Severity != SeverityWarning {
		t.Fatalf("expected warning, got %+v", e)
	}
}

func TestEvaluate_Critical_Both(t *testing.T) {
	e := Evaluate("cow", models.Reading{Temperature: 41.0, HeartRate: 120})
	if !e.Abnormal || e.Severity != SeverityCritical {
		t.Fatalf("expected critical, got %+v", e)
	}
}

func TestThresholdsFor_UnknownSpecies(t *testing.T) {
	got := ThresholdsFor("unknown-species")
	if got != defaultThresholds {
		t.Fatalf("expected default thresholds, got %+v", got)
	}
}
