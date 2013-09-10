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
	// &{[0 0 0] [0 0 0] [0 0 0] [0] [0] 0}
	// &{[0 0 0 0] [0 0 0 0] [0 0 0 0] [0 0 0] [0 0 0] 0}
}
