package pso

import (
	"math"
	"math/rand"
)

type FitnessFunction interface {
	// The main entry point for asking a fitness function questions.
	Query(VecFloat64) float64

	// Produces a random position from the function's domain.
	RandomPos(rgen *rand.Rand) VecFloat64

	// Produces a random velocity suitable for exploring the function's domain.
	RandomVel(rgen *rand.Rand) VecFloat64

	// Compare two fitness values. True if (a <less fit than> b)
	LessFit(a, b float64) bool

	// A suitable size to use as a starting point for a radius calculation.
	RoughDomainDiameter() float64
}

// Sample uniformly from a cube with corners at (min, min, min, ...), (max, max, max, ...)
func UniformCubeSample(dims int, min, max float64, rgen *rand.Rand) (vec VecFloat64) {
	vec = VecFloat64(make([]float64, dims))
	for i := range vec {
		vec[i] = min + rgen.Float64()*(max-min)
	}
	return vec
}

// ----------------------------------------------------------------------
// Parabola/Sphere
// ----------------------------------------------------------------------
type FitnessParabola struct {
	Dims   int
	Center VecFloat64
}

func (f FitnessParabola) Query(pos VecFloat64) float64 {
	s := 0.0
	for i := range pos {
		p := pos[i] - f.Center[i]
		s += p * p
	}
	return s
}

func (f FitnessParabola) RandomPos(rgen *rand.Rand) VecFloat64 {
	return UniformCubeSample(f.Dims, -5.12, 5.12, rgen).Sub(f.Center)
}

func (f FitnessParabola) RandomVel(rgen *rand.Rand) VecFloat64 {
	return UniformCubeSample(f.Dims, -5.12*2, 5.12*2, rgen)
}

func (f FitnessParabola) LessFit(a, b float64) bool {
	return b < a
}

func (f FitnessParabola) RoughDomainDiameter() float64 {
	return math.Sqrt(float64(f.Dims) * (5.12 * 2) * (5.12 * 2))
}

type FitnessSphere FitnessParabola

// ----------------------------------------------------------------------
// Rastrigin
// ----------------------------------------------------------------------
type FitnessRastrigin struct {
	Dims   int
	Center VecFloat64
}

func (f FitnessRastrigin) Query(pos VecFloat64) float64 {
	s := 10.0 * float64(f.Dims)
	for i := range pos {
		p := pos[i] - f.Center[i]
		s += p*p - 10.0*math.Cos(2*math.Pi*p)
	}
	return s
}

func (f FitnessRastrigin) RandomPos(rgen *rand.Rand) VecFloat64 {
	return UniformCubeSample(f.Dims, -5.12, 5.12, rgen).Sub(f.Center)
}

func (f FitnessRastrigin) RandomVel(rgen *rand.Rand) VecFloat64 {
	return UniformCubeSample(f.Dims, -5.12*2, 5.12*2, rgen)
}

func (f FitnessRastrigin) LessFit(a, b float64) bool {
	return b < a
}

func (f FitnessRastrigin) RoughDomainDiameter() float64 {
	return math.Sqrt(float64(f.Dims) * (5.12 * 2) * (5.12 * 2))
}
