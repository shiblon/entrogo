package fitness

import (
	"math"
	"math/rand"
	"monson/vec"
)

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
	RoughDomainDiameter() float64

	// DomainDims returns the number of inputs.
	DomainDims() int
}

// Sample uniformly from a cube with corners at (min, min, min, ...), (max, max, max, ...)
func UniformCubeSample(dims int, min, max float64, rgen *rand.Rand) (v vec.Vec) {
	v = vec.Vec(make([]float64, dims))
	for i := range v {
		v[i] = min + rgen.Float64()*(max-min)
	}
	return v
}

// ----------------------------------------------------------------------
// Parabola/Sphere
// ----------------------------------------------------------------------
type Parabola struct {
	Dims   int
	Center vec.Vec
}

func (f Parabola) DomainDims() int {
	return f.Dims
}

func (f Parabola) Query(pos vec.Vec) float64 {
	s := 0.0
	for i := range pos {
		p := pos[i] - f.Center[i]
		s += p * p
	}
	return s
}

func (f Parabola) RandomPos(rgen *rand.Rand) vec.Vec {
	return UniformCubeSample(f.Dims, -5.12, 5.12, rgen).Sub(f.Center)
}

func (f Parabola) RandomVel(rgen *rand.Rand) vec.Vec {
	return UniformCubeSample(f.Dims, -5.12*2, 5.12*2, rgen)
}

func (f Parabola) LessFit(a, b float64) bool {
	return b < a
}

func (f Parabola) RoughDomainDiameter() float64 {
	return math.Sqrt(float64(f.Dims) * (5.12 * 2) * (5.12 * 2))
}

type Sphere Parabola

// ----------------------------------------------------------------------
// Rastrigin
// ----------------------------------------------------------------------
type Rastrigin struct {
	Dims   int
	Center vec.Vec
}

func (f Rastrigin) DomainDims() int {
	return f.Dims
}

func (f Rastrigin) Query(pos vec.Vec) float64 {
	s := 10.0 * float64(f.Dims)
	for i := range pos {
		p := pos[i] - f.Center[i]
		s += p*p - 10.0*math.Cos(2*math.Pi*p)
	}
	return s
}

func (f Rastrigin) RandomPos(rgen *rand.Rand) vec.Vec {
	return UniformCubeSample(f.Dims, -5.12, 5.12, rgen).Sub(f.Center)
}

func (f Rastrigin) RandomVel(rgen *rand.Rand) vec.Vec {
	return UniformCubeSample(f.Dims, -5.12*2, 5.12*2, rgen)
}

func (f Rastrigin) LessFit(a, b float64) bool {
	return b < a
}

func (f Rastrigin) RoughDomainDiameter() float64 {
	return math.Sqrt(float64(f.Dims) * (5.12 * 2) * (5.12 * 2))
}
