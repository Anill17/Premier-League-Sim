package poisson

import (
	"math"
	"math/rand"
	"testing"
)

// TestSampleNeverNegative is a guard against the cumulative-PMF loop
// going off the rails for unusual inputs.
func TestSampleNeverNegative(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(1))
	for _, lambda := range []float64{0, -1, 0.0001, 1, 2.5, 5, 10, math.NaN()} {
		got := Sample(lambda, r)
		if got < 0 {
			t.Fatalf("Sample(%v) returned negative %d", lambda, got)
		}
	}
}

// TestSampleZeroLambda — λ = 0 must always yield 0.
func TestSampleZeroLambda(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	for i := 0; i < 1000; i++ {
		if got := Sample(0, r); got != 0 {
			t.Fatalf("Sample(0) = %d, want 0", got)
		}
	}
}

// TestSampleMeanApproximatesLambda checks the empirical mean is within
// ~3% of the requested λ for several values. With 50k samples this is
// a very loose bound and false failures should be essentially zero.
func TestSampleMeanApproximatesLambda(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		lambda float64
	}{
		{"lambda_0_5", 0.5},
		{"lambda_1_0", 1.0},
		{"lambda_1_4", 1.4},
		{"lambda_2_5", 2.5},
		{"lambda_4_0", 4.0},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			const n = 50_000
			r := rand.New(rand.NewSource(int64(c.lambda * 1000)))
			var sum int
			for i := 0; i < n; i++ {
				sum += Sample(c.lambda, r)
			}
			mean := float64(sum) / float64(n)
			tolerance := 0.05 // 5%
			if math.Abs(mean-c.lambda) > c.lambda*tolerance+0.05 {
				t.Fatalf("mean %.4f differs from λ %.4f by more than %.0f%%",
					mean, c.lambda, tolerance*100)
			}
		})
	}
}

// TestSampleVarianceApproximatesLambda — for a Poisson(λ) distribution
// the variance equals λ. We allow a 15% deviation because the
// estimator variance is noticeable at n=50k.
func TestSampleVarianceApproximatesLambda(t *testing.T) {
	t.Parallel()

	const (
		lambda    = 2.5
		n         = 50_000
		tolerance = 0.15
	)

	r := rand.New(rand.NewSource(7))
	samples := make([]int, n)
	var sum int
	for i := 0; i < n; i++ {
		samples[i] = Sample(lambda, r)
		sum += samples[i]
	}
	mean := float64(sum) / float64(n)

	var sqDev float64
	for _, s := range samples {
		d := float64(s) - mean
		sqDev += d * d
	}
	variance := sqDev / float64(n-1)

	if math.Abs(variance-lambda) > lambda*tolerance {
		t.Fatalf("variance %.4f differs from λ %.4f by more than %.0f%%",
			variance, lambda, tolerance*100)
	}
}

// TestSamplePMFApproximation compares the empirical PMF against the
// closed-form Poisson PMF for small k. With 50k samples each bucket
// converges to within a couple of percent.
func TestSamplePMFApproximation(t *testing.T) {
	t.Parallel()

	const (
		lambda = 2.0
		n      = 50_000
		eps    = 0.02 // absolute tolerance per bucket
	)

	r := rand.New(rand.NewSource(99))
	counts := make([]int, 10)
	for i := 0; i < n; i++ {
		s := Sample(lambda, r)
		if s < len(counts) {
			counts[s]++
		}
	}

	for k := 0; k < 6; k++ {
		got := float64(counts[k]) / float64(n)
		want := math.Pow(lambda, float64(k)) * math.Exp(-lambda) / factorial(k)
		if math.Abs(got-want) > eps {
			t.Errorf("P(X=%d): got %.4f, want %.4f (Δ=%.4f, tol=%.2f)",
				k, got, want, math.Abs(got-want), eps)
		}
	}
}

// TestSampleDeterministicWithFixedSeed — given the same seed and
// inputs, Sample must produce the same sequence. This is what allows
// tests of the predictor / simulator to be reproducible.
func TestSampleDeterministicWithFixedSeed(t *testing.T) {
	t.Parallel()

	run := func() []int {
		r := rand.New(rand.NewSource(123))
		out := make([]int, 100)
		for i := range out {
			out[i] = Sample(1.7, r)
		}
		return out
	}

	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at index %d: %d vs %d", i, a[i], b[i])
		}
	}
}

// TestPoissonSampleSharedRNG — the no-RNG entry point must still
// produce non-negative values and roughly the right mean. We use a
// loose bound because we cannot seed the shared generator.
func TestPoissonSampleSharedRNG(t *testing.T) {
	t.Parallel()

	const n = 20_000
	var sum int
	for i := 0; i < n; i++ {
		sum += PoissonSample(1.5)
	}
	mean := float64(sum) / float64(n)
	if math.Abs(mean-1.5) > 0.1 {
		t.Fatalf("PoissonSample shared mean %.4f differs from 1.5 by more than 0.1", mean)
	}
}

func factorial(n int) float64 {
	out := 1.0
	for i := 2; i <= n; i++ {
		out *= float64(i)
	}
	return out
}
