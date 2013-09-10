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

	fmt.Printf("p1: %v\n", p1)
	fmt.Printf("p2: %v\n", p2)

	// Output:
	//
	// p1: &{[0 0 0] [0 0 0] [0 0 0] [0] [0] 0}
	// p2: &{[0 0 0 0] [0 0 0 0] [0 0 0 0] [0 0 0] [0 0 0] 0}
}
