// Package sensors simulates physical animal-health sensors. Each virtual
// sensor runs on its own goroutine and timer, emitting readings onto a
// shared channel. This package knows nothing about persistence, rules, or
// HTTP — it only produces models.Reading values.
package sensors

import (
	"context"
	"math/rand"
	"time"

	"livestock/internal/models"
)

// profile describes the normal operating envelope for a species, used to
// generate plausible random readings (with occasional spikes).
type profile struct {
	TempMin, TempMax           float64
	TempSpikeMin, TempSpikeMax float64
	HRMin, HRMax               int
	HRSpikeMin, HRSpikeMax     int
}

var profiles = map[string]profile{
	"cow":     {38.0, 39.5, 40.0, 41.0, 48, 84, 90, 130},
	"sheep":   {38.5, 40.0, 40.5, 41.5, 60, 90, 100, 140},
	"pig":     {38.5, 40.0, 40.5, 41.5, 60, 100, 110, 150},
	"chicken": {40.5, 42.0, 42.2, 43.0, 250, 400, 410, 480},
}

var defaultProfile = profile{38.0, 39.5, 40.0, 41.0, 48, 84, 90, 130}

func profileFor(species string) profile {
	if p, ok := profiles[species]; ok {
		return p
	}
	return defaultProfile
}

// spikeChance is the probability that any given reading is a spike outlier.
const spikeChance = 0.08

// generate produces one plausible random reading for the given animal.
func generate(rng *rand.Rand, animal models.Animal) models.Reading {
	p := profileFor(animal.Species)

	temp := p.TempMin + rng.Float64()*(p.TempMax-p.TempMin)
	hr := p.HRMin + rng.Intn(p.HRMax-p.HRMin+1)

	if rng.Float64() < spikeChance {
		temp = p.TempSpikeMin + rng.Float64()*(p.TempSpikeMax-p.TempSpikeMin)
	}
	if rng.Float64() < spikeChance {
		hr = p.HRSpikeMin + rng.Intn(p.HRSpikeMax-p.HRSpikeMin+1)
	}

	activity := rng.Intn(101)

	return models.Reading{
		AnimalID:    animal.ID,
		Temperature: roundTo(temp, 1),
		HeartRate:   hr,
		Activity:    activity,
		CreatedAt:   time.Now(),
	}
}

func roundTo(v float64, decimals int) float64 {
	mult := 1.0
	for i := 0; i < decimals; i++ {
		mult *= 10
	}
	return float64(int(v*mult+0.5)) / mult
}

// Run starts a single virtual sensor for one animal. It generates a reading
// on a randomized interval (minInterval..maxInterval) and sends it to out,
// until ctx is cancelled.
func Run(ctx context.Context, animal models.Animal, out chan<- models.Reading, minInterval, maxInterval time.Duration) {
	// Independent RNG per goroutine avoids contention on the global rand source.
	rng := rand.New(rand.NewSource(time.Now().UnixNano() ^ int64(animal.ID)))

	nextInterval := func() time.Duration {
		span := maxInterval - minInterval
		if span <= 0 {
			return minInterval
		}
		return minInterval + time.Duration(rng.Int63n(int64(span)))
	}

	timer := time.NewTimer(nextInterval())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			reading := generate(rng, animal)
			select {
			case out <- reading:
			case <-ctx.Done():
				return
			}
			timer.Reset(nextInterval())
		}
	}
}

// StartAll launches one goroutine per animal and returns the shared channel
// they all write readings to. Callers should read from the channel until ctx
// is cancelled.
func StartAll(ctx context.Context, animals []models.Animal, bufferSize int) <-chan models.Reading {
	out := make(chan models.Reading, bufferSize)
	for _, a := range animals {
		go Run(ctx, a, out, 3*time.Second, 8*time.Second)
	}
	return out
}
