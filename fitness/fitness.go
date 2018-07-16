// Package fitness contains standard PSO-evaluation fitness functions from literature.
package fitness

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/shiblon/entrogo/vec"
)

// Function is an interface that can represent any kind of constant-dimension fitness function
// with a concept of random domain sampling.
type Function interface {
	// The main entry point for asking a fitness function questions.
	Query(vec.Vec) float64

	// Produces a random position from the function's domain.
	RandomPos(rgen *rand.Rand) vec.Vec

	// Produces a random velocity suitable for exploring the function's domain.
	RandomVel(rgen *rand.Rand) vec.Vec

	// Compare two fitness values. True if (a <less fit than> b)
	LessFit(a, b float64) bool

	// A suitable size to use as a starting point for a radius calculation.
	Diameter() float64

	// SideLengths returns a vector of positive-valued lengths of the domain rectangle sides.
	SideLengths() vec.Vec

	// Dims returns the number of inputs.
	Dims() int

	// VecInterpreter outputs a string for a vector.
	VecInterpreter(v vec.Vec) string
}

// UniformCubeSample samples uniformly from a cube with corners at (min, min,
// min, ...), (max, max, max, ...).
func UniformCubeSample(dims int, min, max float64, rgen *rand.Rand) (v vec.Vec) {
	v = vec.Vec(make([]float64, dims))
	for i := range v {
		v[i] = min + rgen.Float64()*(max-min)
	}
	return v
}

// UnformHyperrectSample samples uniformly from a hyper rectangle with the given corners.
func UniformHyperrectSample(min, max vec.Vec, rgen *rand.Rand) (v vec.Vec) {
	v = vec.New(len(min))
	for i := range v {
		v[i] = min[i] + rgen.Float64()*(max[i]-min[i])
	}
	return v
}

type QueryFunc func(fit *Fitness, pos vec.Vec) float64

type Fitness struct {
	dims           int
	offsetBy       float64
	minCorner      vec.Vec
	maxCorner      vec.Vec
	sideLengths    vec.Vec
	negSideLengths vec.Vec
	q              QueryFunc

	Center vec.Vec
}

func NewFitness(dims int, minCorner, maxCorner vec.Vec, offsetBy float64, q QueryFunc) *Fitness {
	f := &Fitness{
		dims:        dims,
		minCorner:   minCorner,
		maxCorner:   maxCorner,
		sideLengths: maxCorner.AbsSub(minCorner),
		offsetBy:    offsetBy,
		q:           q,
	}
	f.negSideLengths = f.sideLengths.Negated()
	f.Center = f.sideLengths.SMul(offsetBy)
	return f
}

func NewFitnessSquareDomain(dims int, minDim, maxDim, offsetBy float64, q QueryFunc) *Fitness {
	return NewFitness(dims, vec.NewFilled(dims, minDim), vec.NewFilled(dims, maxDim), offsetBy, q)
}

func (f *Fitness) LessFit(a, b float64) bool {
	return b < a
}

func (f *Fitness) Dims() int {
	return f.dims
}

func (f *Fitness) VecInterpreter(v vec.Vec) string {
	return fmt.Sprintf("%f", []float64(v))
}

func (f *Fitness) RandomPos(rgen *rand.Rand) vec.Vec {
	return UniformHyperrectSample(f.minCorner, f.maxCorner, rgen).Sub(f.Center)
}

func (f *Fitness) RandomVel(rgen *rand.Rand) vec.Vec {
	return UniformHyperrectSample(f.negSideLengths, f.sideLengths, rgen)
}

func (f *Fitness) Diameter() float64 {
	return f.maxCorner.Sub(f.minCorner).Mag()
}

func (f *Fitness) SideLengths() vec.Vec {
	return f.sideLengths
}

func (f *Fitness) Query(pos vec.Vec) float64 {
	return f.q(f, pos)
}

func NewParabola(dims int, offset float64) *Fitness {
	return NewFitnessSquareDomain(dims, -50.0, 50.0, offset, func(f *Fitness, pos vec.Vec) float64 {
		s := 0.0
		for i := range pos {
			p := pos[i] - f.Center[i]
			s += p * p
		}
		return s
	})
}

func NewRastrigin(dims int, offset float64) *Fitness {
	return NewFitnessSquareDomain(dims, -5.12, 5.12, offset, func(f *Fitness, pos vec.Vec) float64 {
		s := 10.0 * float64(f.dims)
		for i := range pos {
			p := pos[i] - f.Center[i]
			s += p*p - 10.0*math.Cos(2*math.Pi*p)
		}
		return s
	})
}

func NewRosenbrock(dims int, offset float64) *Fitness {
	return NewFitnessSquareDomain(dims, -100.0, 100.0, offset, func(f *Fitness, pos vec.Vec) float64 {
		s := 0.0
		for i := 0; i < len(pos)-1; i++ {
			p := pos[i] - f.Center[i]
			p1 := pos[i+1] - f.Center[i+1]
			pinv := (1 - p)
			corr := (p1 - p*p)
			s += pinv*pinv + 100*corr*corr
		}
		return s
	})
}

func NewAckley(dims int, offset float64) *Fitness {
	twopi := 2.0 * math.Pi
	D := float64(dims)
	return NewFitnessSquareDomain(dims, -5.0, 5.0, offset, func(f *Fitness, pos vec.Vec) float64 {
		s1, s2 := 0.0, 0.0
		for i, p := range pos {
			p -= f.Center[i]
			s1 += p * p
			s2 += math.Cos(p * twopi)
		}
		s1 /= D
		s2 /= D
		return -20.0*math.Exp(-0.2*math.Sqrt(s1)) - math.Exp(s2) + 20.0 + math.E
	})
}

func NewDeJongF4(dims int, offset float64) *Fitness {
	return NewFitnessSquareDomain(dims, -20.0, 20.0, offset, func(f *Fitness, pos vec.Vec) float64 {
		s := 0.0
		for i, x := range pos {
			s += float64(i+1) * math.Pow(x-f.Center[i], 4)
		}
		return s
	})
}

func NewEasom(dims int, offset float64) *Fitness {
	return NewFitnessSquareDomain(dims, -100.0, 100.0, offset, func(f *Fitness, pos vec.Vec) float64 {
		sum := 0.0
		prod := 1.0
		for i, x := range pos {
			p := x - f.Center[i]
			sum += p * p
			prod *= math.Cos(math.Pi + p)
		}
		return 1.0 + math.Exp(-sum)*prod
	})
}

func NewSchwefel(dims int, offset float64) *Fitness {
	return NewFitnessSquareDomain(dims, -500.0, 500.0, offset, func(f *Fitness, pos vec.Vec) float64 {
		sum := 0.0
		for i, x := range pos {
			p := x - f.Center[i]
			sum += p * math.Sin(math.Sqrt(math.Abs(p)))
		}
		return 418.9829*float64(f.Dims()) + sum
	})
}
