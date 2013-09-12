package pso

type FitnessFunction interface {
	// The main entry point for asking a fitness function questions.
	Query(VecFloat64) float64

	// Produces a random position from the function's domain.
	RandomPos() VecFloat64

	// Produces a random velocity suitable for exploring the function's domain.
	RandomVel() VecFloat64

	// Compare two fitness values. True if (a <less fit than> b)
	LessFit() (a, b float64) bool
}
