package mealy

// Implement this to specify constraints for the Mealy machine output.
//
// To specify a minimum and/or maximum length, implement IsLargeEnough and/or
// IsSmallEnough, respectively. They work the way you would expect: only values
// that are both small enough and large enough will be emitted.
//
// They are separate functions because they are used in different places for
// different kinds of branch cutting, and this cannot be done properly if the
// two bounds are not specified separately.
//
// If there are only some values that are allowed at certain positions, then
// IsValueAllowed should return true for all allowed values and false for all
// others. If all values are allowed, this must return true all the time.
type Constraints interface {
	IsSmallEnough(size int) bool
	IsLargeEnough(size int) bool
	IsValueAllowed(pos int, val byte) bool
}

// A fully unconstrained Constraints implementation. Always returns true for
// all methods.
type FullyUnconstrained struct{}

func (c FullyUnconstrained) IsSmallEnough(int) bool {
	return true
}
func (c FullyUnconstrained) IsLargeEnough(int) bool {
	return true
}
func (c FullyUnconstrained) IsValueAllowed(int, byte) bool {
	return true
}

