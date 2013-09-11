package pso

import (
	"fmt"
	// "testing"
)

// ----------------------------------------------------------------------
// Test Functions
// ----------------------------------------------------------------------
func ExampleNewParticle() {
	p1 := NewParticle(3)
	p2 := NewParticle(4)

	fmt.Println(p1)
	fmt.Println(p2)

	// Output:
	//
	// &{[0 0 0] [0 0 0] 0 0 [0 0 0] 0 0 [0 0 0] [0 0 0] 0}
	// &{[0 0 0 0] [0 0 0 0] 0 0 [0 0 0 0] 0 0 [0 0 0 0] [0 0 0 0] 0}
}

func ExampleUpdateCurAndBest() {
	p := NewParticle(3)
	fmt.Println(p)
	p.TmpPos = VecFloat64{2, 4, 3}
	p.TmpVal = 10
	fmt.Println(p)
	p.UpdateCur()
	fmt.Println(p)
	p.UpdateBest()
	fmt.Println(p)

	// Output:
	//
	// &{[0 0 0] [0 0 0] 0 0 [0 0 0] 0 0 [0 0 0] [0 0 0] 0}
	// &{[0 0 0] [0 0 0] 0 0 [0 0 0] 0 0 [2 4 3] [0 0 0] 10}
	// &{[2 4 3] [0 0 0] 10 1 [0 0 0] 0 1 [2 4 3] [0 0 0] 10}
	// &{[2 4 3] [0 0 0] 10 1 [2 4 3] 10 0 [2 4 3] [0 0 0] 10}
}
