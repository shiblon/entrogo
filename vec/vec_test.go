package vec

import (
	"fmt"
	"math"
	"testing"
)

func EqualElements(a, b Vec) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestFill(t *testing.T) {
	v1 := Vec{1, 2, 4, 5, 7}
	v1.Fill(2.1)
	if !EqualElements(v1, Vec{2.1, 2.1, 2.1, 2.1, 2.1}) {
		t.Error("Fill is broken")
	}
}

func TestCopy(t *testing.T) {
	v1 := Vec{1, 2, 3, 4}
	v2 := v1.Copy()

	if !EqualElements(v1, v2) {
		t.Error(fmt.Sprintf("Vector %v should be equal to %v", v1, v2))
	}

	if &v1 == &v2 {
		t.Error("Copy did not create a new vector")
	}
}

func TestReplace(t *testing.T) {
	v := Vec{1, 2, 3, 4}
	v.Replace(Vec{2, 3, 2, 3})

	if !EqualElements(v, Vec{2, 3, 2, 3}) {
		t.Error("Failed to Replace")
	}
}

func TestNegate(t *testing.T) {
	v := Vec{1, 2, 3, 4}
	good := Vec{-1, -2, -3, -4}
	v.Negate()

	if !EqualElements(v, good) {
		t.Error(fmt.Sprintf("Vector Negate is broken: %v != %v", v, good))
	}
}

func TestAddBy(t *testing.T) {
	v := Vec{1, 2, 3, 4}
	v.AddBy(Vec{2, 2, 3, 3})

	if !EqualElements(v, Vec{3, 4, 6, 7}) {
		t.Error("Vector AddBy is broken")
	}
}

func TestSubBy(t *testing.T) {
	v := Vec{1, 2, 3, 4}
	v.SubBy(Vec{2, 2, 3, 3})

	if !EqualElements(v, Vec{-1, 0, 0, 1}) {
		t.Error("Vector SubBy is broken")
	}
}

func TestNegated(t *testing.T) {
	vin := Vec{1, 2, 3, 4}
	v := vin.Negated()

	if !EqualElements(v, Vec{-1, -2, -3, -4}) {
		t.Error("Vector Neg is broken")
	}

	if EqualElements(vin, v) {
		t.Error("Original vector should not be changed in Neg")
	}
}

func TestAdd(t *testing.T) {
	vin := Vec{1, 2, 3, 4}
	v := vin.Add(Vec{2, 4, 6, 8})

	if !EqualElements(v, Vec{3, 6, 9, 12}) {
		t.Error("Vector Add is broken")
	}

	if EqualElements(vin, v) {
		t.Error("Original vector should not be changed in Add")
	}
}

func TestSub(t *testing.T) {
	vin := Vec{1, 2, 3, 4}
	v := vin.Sub(Vec{2, 4, 6, 8})

	if !EqualElements(v, Vec{-1, -2, -3, -4}) {
		t.Error("Vector Sub is broken")
	}

	if EqualElements(vin, v) {
		t.Error("Original vector should not be changed in Sub")
	}
}

func ExampleMap() {
	v := Vec{1, 2, 3, 4}
	fmt.Println(v)
	v.MapBy(func(_ int, a float64) float64 { return math.Sqrt(a) })
	x := v.Map(func(i int, a float64) float64 { return float64(i)*a })
	fmt.Println(v)
	fmt.Println(x)

	// Output:
	// [1 2 3 4]
	// [1 1.4142135623730951 1.7320508075688772 2]
	// [0 1.4142135623730951 3.4641016151377544 6]
}

func ExampleNorm() {
	fmt.Println(Vec{1, 2, 3, 4}.Norm(2))
	fmt.Println(Vec{-1, 2, -3, 4}.Norm(1))
	fmt.Println(Vec{-1, 2, -3, 4}.Norm(3))

	// Output:
	// 5.477225575051661
	// 10
	// 4.641588833612779
}

func ExampleDot() {
	fmt.Println(Vec{1, 2, 3, 4}.Dot(Vec{2, 3, 1, 1}))

	// Output:
	// 15
}

func ExampleSAddBy() {
	v := Vec{1, 2, 4, 3}
	fmt.Println(v)
	v.SAddBy(2.5)
	fmt.Println(v)

	// Output:
	// [1 2 4 3]
	// [3.5 4.5 6.5 5.5]
}

func ExampleSSubBy() {
	v := Vec{1, 2, 4, 3}
	fmt.Println(v)
	v.SSubBy(2.5)
	fmt.Println(v)

	// Output:
	// [1 2 4 3]
	// [-1.5 -0.5 1.5 0.5]
}

func ExampleSMulBy() {
	v := Vec{1, 3}
	fmt.Println(v)
	v.SMulBy(3.1)
	fmt.Println(v)

	// Output:
	// [1 3]
	// [3.1 9.3]
}

func ExampleSDivBy() {
	v := Vec{1, 3}
	fmt.Println(v)
	v.SDivBy(2)
	fmt.Println(v)

	// Output:
	// [1 3]
	// [0.5 1.5]
}

func ExampleSAdd() {
	fmt.Println(Vec{1, 5, 2}.SAdd(2))

	// Output:
	// [3 7 4]
}

func ExampleSSub() {
	fmt.Println(Vec{1, 5, 2}.SSub(2))

	// Output:
	// [-1 3 0]
}

func ExampleSMul() {
	fmt.Println(Vec{1, 5, 2}.SMul(2))

	// Output:
	// [2 10 4]
}

func ExampleSDiv() {
	fmt.Println(Vec{1, 5, 2}.SDiv(2))

	// Output:
	// [0.5 2.5 1]
}

func ExampleMag() {
	fmt.Println(Vec{3, 4, -5}.Mag())

	// Output:
	// 7.0710678118654755
}
