package mealy

import (
	"encoding/binary"
	"io"
)

// Must always be 6 bytes.
const serializationPrefix = "MMeMv1"

// Serialize the Mealy machine to a Writer.
func (m MealyMachine) Serialize(w io.Writer) (err error) {
	if err = binary.Write(w, binary.BigEndian, []byte(serializationPrefix)); err != nil {
		return
	}

	if err = binary.Write(w, binary.BigEndian, int32(len(m))); err != nil {
		return
	}

	for _, s := range m {
		if err = binary.Write(w, binary.BigEndian, byte(len(s))); err != nil {
			break
		}
		if err = binary.Write(w, binary.BigEndian, s); err != nil {
			break
		}
	}
	return
}

// Deserialize the Mealy machine from a Reader.
func Deserialize(r io.Reader) (m MealyMachine, err error) {
	// Read version string, then all states in order (each is a slice over
	// uint32).
	versionString := make([]byte, len(serializationPrefix))
	if err = binary.Read(r, binary.BigEndian, versionString); err != nil {
		return
	}

	var numStates int32
	if err = binary.Read(r, binary.BigEndian, &numStates); err != nil {
		return
	}

	m = make(MealyMachine, numStates)
	for i := 0; i < int(numStates); i++ {
		var numTransitions byte
		if err = binary.Read(r, binary.BigEndian, &numTransitions); err != nil {
			return
		}
		st := make(state, numTransitions)
		for t := 0; t < int(numTransitions); t++ {
			var tr transition
			if err = binary.Read(r, binary.BigEndian, &tr); err != nil {
				return
			}
			st[t] = tr
		}
		m[i] = st
	}
	return
}
