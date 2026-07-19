package rules

import "livestock/internal/models"

const (
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Thresholds defines the acceptable range for temperature and heart rate.
type Thresholds struct {
	TempMin, TempMax float64
	HRMin, HRMax     int
}

// bySpecies holds default thresholds per species (bpm ranges are species-dependent).
var bySpecies = map[string]Thresholds{
	"cow":     {TempMin: 38.0, TempMax: 39.5, HRMin: 48, HRMax: 84},
	"sheep":   {TempMin: 38.5, TempMax: 40.0, HRMin: 60, HRMax: 90},
	"pig":     {TempMin: 38.5, TempMax: 40.0, HRMin: 60, HRMax: 100},
	"chicken": {TempMin: 40.5, TempMax: 42.0, HRMin: 250, HRMax: 400},
}

// defaultThresholds is used for any species not explicitly listed.
var defaultThresholds = Thresholds{TempMin: 38.0, TempMax: 39.5, HRMin: 48, HRMax: 84}

// ThresholdsFor returns the threshold table for a given species.
func ThresholdsFor(species string) Thresholds {
	if t, ok := bySpecies[species]; ok {
		return t
	}
	return defaultThresholds
}

// Evaluation is the result of checking a reading against thresholds.
type Evaluation struct {
	Abnormal bool
	Severity string
	Reason   string
}

// Evaluate checks a reading's temperature and heart rate against the species' thresholds.
// If exactly one of temperature/heart-rate is out of range, it's a warning.
// If both are out of range simultaneously, it's critical.
func Evaluate(species string, r models.Reading) Evaluation {
	t := ThresholdsFor(species)

	tempOut := r.Temperature < t.TempMin || r.Temperature > t.TempMax
	hrOut := r.HeartRate < t.HRMin || r.HeartRate > t.HRMax

	switch {
	case tempOut && hrOut:
		return Evaluation{
			Abnormal: true,
			Severity: SeverityCritical,
			Reason:   "High temperature + elevated heart rate",
		}
	case tempOut:
		return Evaluation{
			Abnormal: true,
			Severity: SeverityWarning,
			Reason:   "Temperature out of normal range",
		}
	case hrOut:
		return Evaluation{
			Abnormal: true,
			Severity: SeverityWarning,
			Reason:   "Heart rate out of normal range",
		}
	default:
		return Evaluation{Abnormal: false}
	}
}
