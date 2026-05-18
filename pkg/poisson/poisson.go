// Package poisson provides a small, deterministic-friendly Poisson
// sampler used by the match simulator.
//
// We use inverse-CDF sampling (a.k.a. the cumulative PMF method):
//
//	P(X = 0) = e^{-λ}
//	P(X = k) = P(X = k-1) · λ / k
//
// Draw u ∈ [0,1) and return the smallest k whose cumulative
// probability ≥ u. This is O(λ) and perfectly adequate for football
// xG values (typically λ < 4).
//
// Determinism: all entry points either take a *rand.Rand or accept the
// process-global rand. Tests should always use Sample with their own
// seeded source.
package poisson

import (
	"math"
	"math/rand"
	"sync"
)

// maxIterations caps the cumulative loop so a degenerate λ (e.g. NaN
// or extreme outliers) cannot hang the simulator. 50 covers everything
// a sane xG model will produce.
const maxIterations = 50

// defaultRand is a process-global generator guarded by a mutex. It is
// used only when callers do not bring their own source. Tests must
// always use Sample with a dedicated *rand.Rand.
var (
	defaultRandMu sync.Mutex
	defaultRand   = rand.New(rand.NewSource(rand.Int63()))
)

// PoissonSample draws a single value X ~ Poisson(lambda) using the
// shared process-global generator. For deterministic output (tests,
// Monte Carlo runs that need reproducibility) use Sample.
func PoissonSample(lambda float64) int {
	if lambda <= 0 || math.IsNaN(lambda) {
		return 0
	}
	defaultRandMu.Lock()
	u := defaultRand.Float64()
	defaultRandMu.Unlock()
	return sampleFromUniform(lambda, u)
}

// Sample draws X ~ Poisson(lambda) using the caller-provided generator.
// Pass nil to fall back to PoissonSample's shared generator.
func Sample(lambda float64, r *rand.Rand) int {
	if r == nil {
		return PoissonSample(lambda)
	}
	if lambda <= 0 || math.IsNaN(lambda) {
		return 0
	}
	return sampleFromUniform(lambda, r.Float64())
}

// sampleFromUniform turns a uniform u ∈ [0,1) into a Poisson draw by
// walking the cumulative PMF.
func sampleFromUniform(lambda, u float64) int {
	p := math.Exp(-lambda)
	cumulative := p
	k := 0
	for u > cumulative && k < maxIterations {
		k++
		p = p * lambda / float64(k)
		cumulative += p
	}
	return k
}
