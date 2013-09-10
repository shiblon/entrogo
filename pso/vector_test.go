package pso

import (
	"fmt"
	"math"
	"testing"
)

func EqualElements(a, b VecFloat64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFill(t *testing.T) {
	v1 := VecFloat64{1, 2, 4, 5, 7}
	v1.Fill(2.1)
	if !EqualElements(v1, VecFloat64{2.1, 2.1, 2.1, 2.1, 2.1}) {
		t.Error("Fill is broken")
	}
}

func TestCopy(t *testing.T) {
	v1 := VecFloat64{1, 2, 3, 4}
	v2 := v1.Copy()

	if !EqualElements(v1, v2) {
		t.Error(fmt.Sprintf("Vector %v should be equal to %v", v1, v2))
	}

	if &v1 == &v2 {
		t.Error("Copy did not create a new vector")
	}
}

func TestReplace(t *testing.T) {
	v := VecFloat64{1, 2, 3, 4}
	v.Replace(VecFloat64{2, 3, 2, 3})

	if !EqualElements(v, VecFloat64{2, 3, 2, 3}) {
		t.Error("Failed to Replace")
	}
}

func TestNegate(t *testing.T) {
	v := VecFloat64{1, 2, 3, 4}
	v.Negate()

	if !EqualElements(v, VecFloat64{-1, -2, -3, -4}) {
		t.Error("Vector Negate is broken")
	}
}

func TestIncrBy(t *testing.T) {
	v := VecFloat64{1, 2, 3, 4}
	v.IncrBy(VecFloat64{2, 2, 3, 3})

	if !EqualElements(v, VecFloat64{3, 4, 6, 7}) {
		t.Error("Vector IncrBy is broken")
	}
}

func TestDecrBy(t *testing.T) {
	v := VecFloat64{1, 2, 3, 4}
	v.DecrBy(VecFloat64{2, 2, 3, 3})

	if !EqualElements(v, VecFloat64{-1, 0, 0, 1}) {
		t.Error("Vector DecrBy is broken")
	}
}

func TestNeg(t *testing.T) {
	vin := VecFloat64{1, 2, 3, 4}
	v := vin.Neg()

	if !EqualElements(v, VecFloat64{-1, -2, -3, -4}) {
		t.Error("Vector Neg is broken")
	}

	if EqualElements(vin, v) {
		t.Error("Original vector should not be changed in Neg")
	}
}

func TestAdd(t *testing.T) {
	vin := VecFloat64{1, 2, 3, 4}
	v := vin.Add(VecFloat64{2, 4, 6, 8})

	if !EqualElements(v, VecFloat64{3,6,9,12}) {
		t.Error("Vector Add is broken")
	}

	if EqualElements(vin, v) {
		t.Error("Original vector should not be changed in Add")
	}
}

func TestSub(t *testing.T) {
	vin := VecFloat64{1, 2, 3, 4}
	v := vin.Sub(VecFloat64{2, 4, 6, 8})

	if !EqualElements(v, VecFloat64{-1, -2, -3, -4}) {
		t.Error("Vector Sub is broken")
	}

	if EqualElements(vin, v) {
		t.Error("Original vector should not be changed in Sub")
	}
}

func ExampleMap() {
	v := VecFloat64{1, 2, 3, 4}
	fmt.Println(v)
	v.MapBy(func(a float64) float64 {return math.Sqrt(a)})
	fmt.Println(v)

	// Output:
	// [1 2 3 4]
	// [1 1.4142135623730951 1.7320508075688772 2]
}

func ExampleNorm() {
	fmt.Println(VecFloat64{1, 2, 3, 4}.Norm(2))
	fmt.Println(VecFloat64{-1, 2, -3, 4}.Norm(1))
	fmt.Println(VecFloat64{-1, 2, -3, 4}.Norm(3))

	// Output:
	// 5.477225575051661
	// 10
	// 4.641588833612779
}

func ExampleDot() {
	fmt.Println(VecFloat64{1, 2, 3, 4}.Dot(VecFloat64{2, 3, 1, 1}))

	// Output:
	// 15
}

func ExampleSIncrBy() {
	v := VecFloat64{1, 2, 4, 3}
	fmt.Println(v)
	v.SIncrBy(2.5)
	fmt.Println(v)

	// Output:
	// [1 2 4 3]
	// [3.5 4.5 6.5 5.5]
}

func ExampleSDecrBy() {
	v := VecFloat64{1, 2, 4, 3}
	fmt.Println(v)
	v.SDecrBy(2.5)
	fmt.Println(v)

	// Output:
	// [1 2 4 3]
	// [-1.5 -0.5 1.5 0.5]
}

func ExampleSMulBy() {
	v := VecFloat64{1, 3}
	fmt.Println(v)
	v.SMulBy(3.1)
	fmt.Println(v)

	// Output:
	// [1 3]
	// [3.1 9.3]
}

func ExampleSDivBy() {
	v := VecFloat64{1, 3}
	fmt.Println(v)
	v.SDivBy(2)
	fmt.Println(v)

	// Output:
	// [1 3]
	// [0.5 1.5]
}

func ExampleSAdd() {
	fmt.Println(VecFloat64{1, 5, 2}.SAdd(2))

	// Output:
	// [3 7 4]
}

func ExampleSSub() {
	fmt.Println(VecFloat64{1, 5, 2}.SSub(2))

	// Output:
	// [-1 3 0]
}

func ExampleSMul() {
	fmt.Println(VecFloat64{1, 5, 2}.SMul(2))

	// Output:
	// [2 10 4]
}

func ExampleSDiv() {
	fmt.Println(VecFloat64{1, 5, 2}.SDiv(2))

	// Output:
	// [0.5 2.5 1]
}
