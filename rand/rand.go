// Package rand contains facilities for generating random numbers, with various implementations.
package rand

type Rand interface {
	Float64() float64
	NormFloat64() float64
}
