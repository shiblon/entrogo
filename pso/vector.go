package pso

import (
	"fmt"
	"math"
	"math/rand"
)

type VecFloat64 []float64

func NewUniformVecFloat64(size int) VecFloat64 {
	vec := VecFloat64(make([]float64, size))
	vec.FillStandardUniform()
	return vec
}

func (vec *VecFloat64) Fill(val float64) *VecFloat64 {
	for i := range *vec {
		(*vec)[i] = val
	}
	return vec
}

func (vec *VecFloat64) FillUniform() *VecFloat64 {
	for i := range *vec {
		(*vec)[i] = rand.Float64()
	}
	return vec
}

func (vec VecFloat64) Copy() VecFloat64 {
	out := VecFloat64(make([]float64, len(vec)))
	copy(out, vec)
	return out
}

func (vec *VecFloat64) Replace(other VecFloat64) *VecFloat64 {
	copy(*vec, other)
	return vec
}

func (vec *VecFloat64) Negate() *VecFloat64 {
	for i := range *vec {
		(*vec)[i] = -(*vec)[i]
	}
	return vec
}

func (vec *VecFloat64) IncrBy(other VecFloat64) *VecFloat64 {
	if len(*vec) != len(other) {
		panic(fmt.Sprintf("cannot add vectors of different lengths: %d != %d", len(*vec), len(other)))
	}

	for i := range *vec {
		(*vec)[i] += other[i]
	}
	return vec
}

func (vec *VecFloat64) DecrBy(other VecFloat64) *VecFloat64 {
	if len(*vec) != len(other) {
		panic(fmt.Sprintf("cannot subtract vectors of different lengths: %d != %d", len(*vec), len(other)))
	}

	for i := range *vec {
		(*vec)[i] -= other[i]
	}
	return vec
}

func (vec *VecFloat64) MulBy(other VecFloat64) *VecFloat64 {
	if len(*vec) != len(other) {
		panic(fmt.Sprintf("cannot multiply vectors of different lengths: %d != %d", len(*vec), len(other)))
	}

	for i := range *vec {
		(*vec)[i] *= other[i]
	}
	return vec
}

func (vec *VecFloat64) DivBy(other VecFloat64) *VecFloat64 {
	if len(*vec) != len(other) {
		panic(fmt.Sprintf("cannot divide vectors of different lengths: %d != %d", len(*vec), len(other)))
	}

	for i := range *vec {
		(*vec)[i] /= other[i]
	}
	return vec
}

func (vec *VecFloat64) MapBy(mapfunc func (float64) float64) *VecFloat64 {
	for i := range *vec {
		(*vec)[i] = mapfunc((*vec)[i])
	}
	return vec
}

// Scalar increment
func (vec *VecFloat64) SIncrBy(s float64) *VecFloat64 {
	for i := range *vec {
		(*vec)[i] += s
	}
	return vec
}

// Scalar decrement
func (vec *VecFloat64) SDecrBy(s float64) *VecFloat64 {
	for i := range *vec {
		(*vec)[i] -= s
	}
	return vec
}

// Scalar multiply
func (vec *VecFloat64) SMulBy(s float64) *VecFloat64 {
	for i := range *vec {
		(*vec)[i] *= s
	}
	return vec
}

// Scalar divide
func (vec *VecFloat64) SDivBy(s float64) *VecFloat64 {
	for i := range *vec {
		(*vec)[i] /= s
	}
	return vec
}

func (vec VecFloat64) Neg() VecFloat64 {
	out := vec.Copy()
	out.Negate()
	return out
}

func (vec VecFloat64) Add(other VecFloat64) VecFloat64 {
	out := vec.Copy()
	out.IncrBy(other)
	return out
}

func (vec VecFloat64) Mul(other VecFloat64) VecFloat64 {
	out := vec.Copy()
	out.MulBy(other)
	return out
}

func (vec VecFloat64) Div(other VecFloat64) VecFloat64 {
	out := vec.Copy()
	out.DivBy(other)
	return out
}

func (vec VecFloat64) SAdd(s float64) VecFloat64 {
	out := vec.Copy()
	out.SIncrBy(s)
	return out
}

func (vec VecFloat64) SSub(s float64) VecFloat64 {
	out := vec.Copy()
	out.SDecrBy(s)
	return out
}

func (vec VecFloat64) SMul(s float64) VecFloat64 {
	out := vec.Copy()
	out.SMulBy(s)
	return out
}

func (vec VecFloat64) SDiv(s float64) VecFloat64 {
	out := vec.Copy()
	out.SDivBy(s)
	return out
}

func (vec VecFloat64) Sub(other VecFloat64) VecFloat64 {
	out := vec.Copy()
	out.DecrBy(other)
	return out
}

func (vec VecFloat64) Map(mapfunc func (float64) float64) VecFloat64 {
	out := vec.Copy()
	out.MapBy(mapfunc)
	return out
}

func (vec VecFloat64) Norm(degree float64) float64 {
	if degree <= 0.0 {
		panic(fmt.Sprintf("Invalid non-positive norm degree: %v", degree))
	}
	s := 0.0
	// Special case for plain sum
	if degree == 1.0 {
		for i := range vec {
			s += math.Abs(vec[i])
		}
		return s
	}

	for i := range vec {
		s += math.Pow(math.Abs(vec[i]), degree)
	}
	return math.Pow(s, 1.0/degree)
}

func (vec VecFloat64) Dot(other VecFloat64) float64 {
	if len(vec) != len(other) {
		panic(fmt.Sprintf("cannot dot vectors of different lengths: %d != %d", len(vec), len(other)))
	}

	s := 0.0
	for i := range vec {
		s += vec[i] * other[i]
	}
	return s
}
