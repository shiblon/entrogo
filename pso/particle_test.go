package pso

import (
	"fmt"
	// "testing"
)

// ----------------------------------------------------------------------
// Test Functions
// ----------------------------------------------------------------------
func ExampleNewParticle() {
	p1 := NewParticle(3, 1)
	p2 := NewParticle(4, 3)

	fmt.Println(p1)
	fmt.Println(p2)

	// Output:
	//
	// &{[0 0 0] [0 0 0] [0] [0 0 0] [0] 0 [0 0 0] [0 0 0] [0]}
	// &{[0 0 0 0] [0 0 0 0] [0 0 0] [0 0 0 0] [0 0 0] 0 [0 0 0 0] [0 0 0 0] [0 0 0]}
}

func ExampleUpdateCurAndBest() {
	p := NewParticle(3, 2)
	fmt.Println(p)
	p.TmpPos = VecFloat64{2, 4, 3}
	p.TmpVal = VecFloat64{10, 12}
	fmt.Println(p)
	p.UpdateCur()
	fmt.Println(p)
	p.UpdateBest()
	fmt.Println(p)

	// Output:
	//
	// &{[0 0 0] [0 0 0] [0 0] [0 0 0] [0 0] 0 [0 0 0] [0 0 0] [0 0]}
	// &{[0 0 0] [0 0 0] [0 0] [0 0 0] [0 0] 0 [2 4 3] [0 0 0] [10 12]}
	// &{[2 4 3] [0 0 0] [10 12] [0 0 0] [0 0] 1 [2 4 3] [0 0 0] [10 12]}
	// &{[2 4 3] [0 0 0] [10 12] [2 4 3] [10 12] 0 [2 4 3] [0 0 0] [10 12]}
}
