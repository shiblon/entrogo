package pso

import (
	"fmt"
	"monson/pso/vec"
)

// ----------------------------------------------------------------------
// Test Functions
// ----------------------------------------------------------------------
func ExampleNewParticle() {
	p1 := NewParticle(vec.Vec{0, 0, 0}, vec.Vec{0, 0, 0})
	p2 := NewParticle(vec.Vec{0, 0, 0, 0}, vec.Vec{0, 0, 0, 0})

	fmt.Println(p1)
	fmt.Println(p2)

	// Output:
	//
	// &{[0 0 0] [0 0 0] 0 0 [0 0 0] 0 0 [0 0 0] [0 0 0] 0}
	// &{[0 0 0 0] [0 0 0 0] 0 0 [0 0 0 0] 0 0 [0 0 0 0] [0 0 0 0] 0}
}

func ExampleUpdateCurAndBest() {
	p := NewParticle(vec.Vec{0, 0, 0}, vec.Vec{0, 0, 0})
	fmt.Println(p)
	p.TmpPos = vec.Vec{2, 4, 3}
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
