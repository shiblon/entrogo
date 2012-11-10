package mealy

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	//"testing/quick"
)

type TestStrings []string

func (t TestStrings) ToChannel() <-chan []byte {
	sch := make(chan []byte)
	go func() {
		defer close(sch)
		for _, s := range t {
			sch <- []byte(s)
		}
	}()
	return sch
}

func AllStrings() TestStrings {
	return TestStrings{
		"A",
		"AA",
		"AAA",
		"AAB",
		"BAA",
		"CBA",
		"CBB",
		"DABBER",
		"DOBBER",
	}
}

type SizeConstraint TestStrings

func (c SizeConstraint) IsLargeEnough(s int) bool          { return s >= 2 }
func (c SizeConstraint) IsSmallEnough(s int) bool          { return s <= 3 }
func (c SizeConstraint) IsValueAllowed(i int, v byte) bool { return true }

func SizeConstrainedStrings() SizeConstraint {
	return SizeConstraint{
		"AA",
		"AAA",
		"AAB",
		"BAA",
		"CBA",
		"CBB",
	}
}

type A1Constraint TestStrings

func (c A1Constraint) IsLargeEnough(s int) bool { return true }
func (c A1Constraint) IsSmallEnough(s int) bool { return true }
func (c A1Constraint) IsValueAllowed(i int, v byte) bool {
	return i != 1 || v == byte('A')
}

func A1ConstrainedStrings() A1Constraint {
	return A1Constraint{
		"A", // Allowed because we aren't testing for length.
		"AA",
		"AAA",
		"AAB",
		"BAA",
		"DABBER",
	}
}

type A1SizeConstraint TestStrings

func (c A1SizeConstraint) IsLargeEnough(s int) bool { return s >= 2 }
func (c A1SizeConstraint) IsSmallEnough(s int) bool { return s <= 3 }
func (c A1SizeConstraint) IsValueAllowed(i int, v byte) bool {
	return i != 1 || v == byte('A')
}

func A1SizeConstrainedStrings() A1SizeConstraint {
	return A1SizeConstraint{
		"AA",
		"AAA",
		"AAB",
		"BAA",
	}
}

func EqualChannels(t *testing.T, c1, c2 <-chan []byte) error {
	var o, c []byte
	oOk, cOk := true, true
	for oOk && cOk {
		o, oOk = <-c1
		c, cOk = <-c2
		t.Logf("Expected '%v':%t, Received '%v':%t\n", string(o), oOk, string(c), cOk)
		if oOk != cOk {
			return errors.New(fmt.Sprintf(
				"Channels closed at different times: %t:%t\n", oOk, cOk))
		}
		if bytes.Compare(o, c) != 0 {
			return errors.New(fmt.Sprintf(
				"Channels had different data: %v:%v\n", o, c))
		}
	}
	return nil
}

// ----------------------------------------------------------------------
// Test Functions
// ----------------------------------------------------------------------
func ExampleRecognizes() {
	m := NewMealyMachine(AllStrings().ToChannel())

	fmt.Println(m.Recognizes([]byte("BAA")))
	fmt.Println(m.Recognizes([]byte("CBB")))
	fmt.Println(m.Recognizes([]byte("DABB")))

	// Output:
	// true
	// true
	// false
}

func TestAllSequences(t *testing.T) {
	strings := AllStrings()
	m := NewMealyMachine(strings.ToChannel())
	if err := EqualChannels(t, strings.ToChannel(), m.AllSequences()); err != nil {
		t.Error(err.Error())
	}
}

func TestSizeConstrainedSequences(t *testing.T) {
	m := NewMealyMachine(AllStrings().ToChannel())
	con := SizeConstrainedStrings()
	if err := EqualChannels(t, TestStrings(con).ToChannel(), m.ConstrainedSequences(con)); err != nil {
		t.Error(err.Error())
	}
}

func TestA1ConstrainedSequences(t *testing.T) {
	m := NewMealyMachine(AllStrings().ToChannel())
	con := A1ConstrainedStrings()
	if err := EqualChannels(t, TestStrings(con).ToChannel(), m.ConstrainedSequences(con)); err != nil {
		t.Error(err.Error())
	}
}

func TestA1SizeConstrainedSequences(t *testing.T) {
	m := NewMealyMachine(AllStrings().ToChannel())
	con := A1SizeConstrainedStrings()
	if err := EqualChannels(t, TestStrings(con).ToChannel(), m.ConstrainedSequences(con)); err != nil {
		t.Error(err.Error())
	}
}
