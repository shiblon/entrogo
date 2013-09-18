// Vector math package for common vector operations and a vector data type.
package vec

import (
	"fmt"
	"math"
)

type Vec []float64

func assertSameLen(a, b Vec) {
	if len(a) != len(b) {
		panic(fmt.Sprintf("Vectors are not the same length: %v  %v", a, b))
	}
}

// New creates a new Vec of the given dimensionality.
func New(size int) Vec {
	return Vec(make([]float64, size))
}

// NewFilled creates and initializes the vector so all elements are the specified value.
func NewFilled(size int, val float64) Vec {
	return New(size).Fill(val)
}

// NewFFilled creates and initializes the vector so all elements come from f().
func NewFFilled(size int, f func() float64) Vec {
	return New(size).FFill(f)
}

// Fill changes all vector values to the given value.
func (v Vec) Fill(val float64) Vec {
	for i := range v {
		v[i] = val
	}
	return v
}

// FFill changes all values to contain f().
func (v Vec) FFill(f func() float64) Vec {
	for i := range v {
		v[i] = f()
	}
	return v
}

// Replace copies all elements from other into this vector.
func (v Vec) Replace(other Vec) Vec {
	assertSameLen(v, other)
	copy(v, other)
	return v
}

// Copy returns a new, identical vector with its own underlying memory.
func (v Vec) Copy() Vec {
	return New(len(v)).Replace(v)
}

// Negated returns a new vector where all new[i] = -v[i].
func (v Vec) Negated() Vec {
	return v.Copy().Negate()
}

// Negate changes all v[i] to -v[i].
func (v Vec) Negate() Vec {
	return v.SMulBy(-1)
}

// AddBy changes every value v[i] to be the v[i] + other[i].
func (v Vec) AddBy(other Vec) Vec {
	assertSameLen(v, other)
	for i, val := range other {
		v[i] += val
	}
	return v
}

// SubBy changes every value v[i] to be v[i] - other[i].
func (v Vec) SubBy(other Vec) Vec {
	assertSameLen(v, other)
	for i, val := range other {
		v[i] -= val
	}
	return v
}

// MulBy changes every value v[i] to be v[i] * other[i].
func (v Vec) MulBy(other Vec) Vec {
	assertSameLen(v, other)
	for i, val := range other {
		v[i] *= val
	}
	return v
}

// DivBy changes every value v[i] to v[i] / other[i].
func (v Vec) DivBy(other Vec) Vec {
	assertSameLen(v, other)
	for i, val := range other {
		v[i] /= val
	}
	return v
}

// MapBy replaces all v[i] with f(i, v[i]).
func (v Vec) MapBy(f func(int, float64) float64) Vec {
	for i, val := range v {
		v[i] = f(i, val)
	}
	return v
}

// SAddBy replaces all v[i] with v[i] + val.
func (v Vec) SAddBy(val float64) Vec {
	for i := range v {
		v[i] += val
	}
	return v
}

// SSubBy replaces all v[i] with v[i] - val.
func (v Vec) SSubBy(val float64) Vec {
	for i := range v {
		v[i] -= val
	}
	return v
}

// SMulBy replaces all v[i] with v[i] * val.
func (v Vec) SMulBy(val float64) Vec {
	for i := range v {
		v[i] *= val
	}
	return v
}

// SDivBy replaces all v[i] with v[i] / val.
func (v Vec) SDivBy(val float64) Vec {
	for i := range v {
		v[i] /= val
	}
	return v
}

// Add returns a new vector with all elements equal to v[i] + other[i].
func (v Vec) Add(other Vec) Vec {
	return v.Copy().AddBy(other)
}

// Sub returns a new vector with all elements equal to v[i] - other[i].
func (v Vec) Sub(other Vec) Vec {
	return v.Copy().SubBy(other)
}

// Mul returns a new vector with all elements equal to v[i] * other[i].
func (v Vec) Mul(other Vec) Vec {
	return v.Copy().MulBy(other)
}

// Div returns a new vector with all elements equal to v[i] / other[i].
func (v Vec) Div(other Vec) Vec {
	return v.Copy().DivBy(other)
}

// SAdd returns a new vector with all elements equal to v[i] + val.
func (v Vec) SAdd(val float64) Vec {
	return v.Copy().SAddBy(val)
}

// SSub returns a new vector with all elements equal to v[i] + val.
func (v Vec) SSub(val float64) Vec {
	return v.Copy().SSubBy(val)
}

// SMul returns a new vector with all elements equal to v[i] + val.
func (v Vec) SMul(val float64) Vec {
	return v.Copy().SMulBy(val)
}

// SDiv returns a new vector with all elements equal to v[i] + val.
func (v Vec) SDiv(val float64) Vec {
	return v.Copy().SDivBy(val)
}

// Map returns a new vector with all elements equal to f(i, v[i]).
func (v Vec) Map(f func(int, float64) float64) Vec {
	return v.Copy().MapBy(f)
}

// Norm returns the degree-norm of the vector. The 2-norm is the euclidean distance from the origin.
// The degree must be positive.
func (v Vec) Norm(degree float64) float64 {
	if degree <= 0.0 {
		panic(fmt.Sprintf("Invalid non-positive norm degree: %v", degree))
	}
	s := 0.0
	// Special case for plain sum
	if degree == 1.0 {
		for _, val := range v {
			s += math.Abs(val)
		}
		return s
	}

	for _, val := range v {
		s += math.Pow(math.Abs(val), degree)
	}
	return math.Pow(s, 1.0/degree)
}

// Dot returns the dot product of v and other.
func (v Vec) Dot(other Vec) float64 {
	assertSameLen(v, other)
	s := 0.0
	for i, val := range other {
		s += v[i] * val
	}
	return s
}

// Mag returns the 2-norm of the vector. It is potentially a bit more efficient than calling Norm(2).
func (v Vec) Mag() float64 {
	return math.Sqrt(v.Dot(v))
}
