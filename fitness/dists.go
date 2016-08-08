package fitness

import (
	"math"
	"math/rand"
)

const (
	Sqrt_2_pi = 2.5066282746310002 // sqrt(2 * pi)
	Log_pi    = 1.1447298858494002 // log(pi)
	MinProb   = 1e-10
)

// Snorm computes a single sample from a one-dimensional normal distribution.
func Snorm(mu, sigma float64, rgen *rand.Rand) float64 {
	return rgen.NormFloat64()*sigma + mu
}

func lognormParamsFromNormParams(mean, stddev float64) (float64, float64) {
	v := stddev * stddev
	m2 := mean * mean
	return math.Log(m2 / math.Sqrt(v+m2)), math.Sqrt(math.Log(1 + v/m2))
}

// Slognorm computes a single sample from a one-dimensional log normal distribution.
func Slognorm(mu, sigma float64, rgen *rand.Rand) float64 {
	// The logarithm of x is distributed normally with these parameters, so we
	// get a normal draw and convert it back out of log space.
	return math.Exp(mu + sigma*Snorm(0, 1, rgen))
}

// Dnorm computes the normal density with the given parameters.
func Dnorm(x, mu, sigma float64) float64 {
	d := x - mu
	return math.Exp(-d*d/(2.0*sigma*sigma)) / (sigma * math.Sqrt(2.0*math.Pi))
}

// LogDnorm computes the log of the normal density with the given parameters.
func LogDnorm(x, mu, sigma float64) float64 {
	d := x - mu
	return -d*d/(2.0*sigma*sigma) - math.Log(sigma*Sqrt_2_pi)
}

// Fdtrunc2norm creates a function that computes the density of a normal distribution truncated on both sides.
func Fdtrunc2norm(minx, maxx, mu, sigma float64) func(x float64) float64 {
	goodMass := Cnorm(maxx, mu, sigma) - Cnorm(minx, mu, sigma)
	return func(x float64) float64 {
		if x < minx {
			return MinProb
		} else if x > maxx {
			return MinProb
		} else {
			return Dnorm(x, mu, sigma) / goodMass
		}
	}
}

// Erf is the standard Error Function.
func Erf(x float64) (erf float64) {
	t := 1.0 / (1.0 + 0.5*math.Abs(x))
	t2 := t * t
	t3 := t2 * t
	t4 := t3 * t
	t5 := t4 * t
	t6 := t5 * t
	t7 := t6 * t
	t8 := t7 * t
	t9 := t8 * t

	tau := t * math.Exp(
		-x*x-
			1.26551223+1.00002368*t+0.37409196*t2+0.09678418*t3-
			0.18628806*t4+0.27886807*t5-1.13520398*t6+1.48851587*t7-
			0.82215223*t8+0.17087277*t9)

	erf = tau - 1.0
	if x >= 0 {
		erf = -erf
	}
	return
}

// Cnorm is the cumulative distribution function for the Gaussian.
func Cnorm(x, mu, sigma float64) float64 {
	x = (x - mu) / sigma
	return (1.0 + Erf(x/math.Sqrt(2))) / 2.0
}

// Dlognorm computes the density of the log normal at the given location.
func Dlognorm(x, mu, sigma float64) float64 {
	d := math.Log(x) - mu
	return math.Exp(-d*d/(2*sigma*sigma)) / (x * sigma * math.Sqrt(2*math.Pi))
}

// Dcauchy computes the density of the one-dimensional Cauchy at the given location.
func Dcauchy(x, x0, gamma float64) float64 {
	d := x - x0
	return 1.0 / (math.Pi * (gamma + d*d/gamma))
}

// LogDcauchy computes the log fo the one-dimensional Cauchy at the given location.
func LogDcauchy(x, x0, gamma float64) float64 {
	d := x - x0
	return -Log_pi - math.Log(gamma+(d*d)/gamma)
}

// Ccauchy computes the CDF.
func Ccauchy(x, x0, gamma float64) float64 {
	return math.Atan((x-x0)/gamma)/math.Pi + .5
}

// Sigmoid computes the value of the sigmoid function with the given scaling and shifting.
func Sigmoid(x, x0, xs, ys float64) float64 {
	return ys / (1.0 + math.Exp((x0-x)/xs))
}
