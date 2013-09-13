package pso

import (
	"math/rand"
)

type FitnessFunction interface {
	// The main entry point for asking a fitness function questions.
	Query(VecFloat64) float64

	// Produces a random position from the function's domain.
	RandomPos() VecFloat64

	// Produces a random velocity suitable for exploring the function's domain.
	RandomVel() VecFloat64

	// Compare two fitness values. True if (a <less fit than> b)
	LessFit(a, b float64) bool
}

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

func (f FitnessParabola) RandomPos() VecFloat64 {
	maxval := 5.12
	pos := VecFloat64(make([]float64, f.Dims))
	for i := range pos {
		pos[i] = maxval * (2*rand.Float64() - 1) - f.Center[i]
	}
	return pos
}

func (f FitnessParabola) RandomVel() VecFloat64 {
	maxval := 5.12
	vel := VecFloat64(make([]float64, f.Dims))
	for i := range vel {
		vel[i] = 2 * maxval * (2*rand.Float64() - 1)
	}
	return vel
}

func (f FitnessParabola) LessFit(a, b float64) bool {
	return b < a
}
