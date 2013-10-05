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
//
// Finally, you can always just implement IsSequenceAllowed, which passes the
// whole sequence to you before emission, and you can test the whole thing for
// compliance in whatever way you choose. You could technically implement any
// constraint this way, but be advised that *only* using this function will not
// be as efficient as size- and branch-bounding things using the other
// functions, because it will have to traverse every path in the automaton. It
// can be very useful as a finishing touch to the others, however, since they
// are less general.
type Constraints interface {
	IsSmallEnough(size int) bool
	IsLargeEnough(size int) bool
	IsValueAllowed(pos int, val byte) bool
	IsSequenceAllowed(seq []byte) bool
}

// A fully unconstrained Constraints implementation. Always returns true for
// all methods. It is (very) safe to "inherit" from this, since it has no internal state, to create your own Constraints-compatible types without having to specify all methods, e.g.,
//
// type NotTooLarge struct {
// 		BaseConstraints
// }
//
// func (n NotTooLarge) IsSmallEnough(size int) bool {
// 		return size < 5
// }
type BaseConstraints struct{}

func (c BaseConstraints) IsSmallEnough(int) bool {
	return true
}
func (c BaseConstraints) IsLargeEnough(int) bool {
	return true
}
func (c BaseConstraints) IsValueAllowed(int, byte) bool {
	return true
}
func (c BaseConstraints) IsSequenceAllowed([]byte) bool {
	return true
}
