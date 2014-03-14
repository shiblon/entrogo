package particle

import (
	"fmt"
	"monson/vec"
)

// ----------------------------------------------------------------------
// Test Functions
// ----------------------------------------------------------------------
func ExampleNewParticle() {
	p1 := NewParticle()
	p1.Init(vec.Vec{1, 2, 3}, vec.Vec{0, 0, 0}, 1.1)
	p2 := NewParticle()
	p2.Init(vec.Vec{2, 3, 4, 5}, vec.Vec{0, 0, 0, 0}, 2.2)

	fmt.Println(p1)
	fmt.Println(p2)

	// Output:
	//
	// &{[0 0 0] [0 0 0] 0 0 [0 0 0] 0 0 [0 0 0] [0 0 0] 0}
	// &{[0 0 0 0] [0 0 0 0] 0 0 [0 0 0 0] 0 0 [0 0 0 0] [0 0 0 0] 0}
}

func ExampleUpdateCurAndBest() {
	p := NewParticle()
	p.Init(vec.Vec{0, 0, 0}, vec.Vec{0, 0, 0})
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
