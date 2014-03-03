// Copyright 2014 Chris Monson <shiblon@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/* Package journal is an implementation and interface specification for an
* append-only journal with rotations. It contains a few simple implementations,
* as well.
*/
package journal

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

type Interface interface {
	// Append appends a serialized version of the interface to the
	// current journal shard.
	Append(interface{}) error

	// StartSnapshot is given a data channel from which it is expected to
	// consume all values until closed. If it terminates early, it sends a
	// non-nil error back. When complete with no errors, the snapshot has been
	// successfully processed. Whether the current shard is full or not, this
	// function immediately trigger a shard rotation so that subsequent calls
	// to Append go to a new shard.
	StartSnapshot(records <-chan interface{}, resp <-chan error) error

	// SnapshotDecoder returns a decode function that can be called to decode
	// the next element in the most recent snapshot.
	SnapshotDecoder() (func(interface{}) error, error)

	// JournalDecoder returns a decode function that can be called to decode
	// the next element in the journal stream.
	JournalDecoder() (func(interface{}) error, error)
}

type Decoder interface {
	Decode(interface{}) error
}

// TODO(chris): add the Decoder stuff to the implementations below.

type Bytes struct {
	enc  *gob.Encoder
	buff *bytes.Buffer
}

func NewBytes() *Bytes{
	j := &Bytes{
		buff: new(bytes.Buffer),
	}
	j.enc = gob.NewEncoder(j.buff)
	return j
}

func (j Bytes) ShardFinished() bool {
	return false
}

func (j Bytes) Append(rec interface{}) error {
	return j.enc.Encode(rec)
}

func (j Bytes) Bytes() []byte {
	return j.buff.Bytes()
}

func (j Bytes) String() string {
	return j.buff.String()
}

func (j *Bytes) StartSnapshot(records <-chan interface{}, snapresp <-chan error) error {
	go func() {
		snapresp <- nil
	}()
	return nil
}

type Count int64

func NewCount() *Count {
	return new(Count)
}

func (j Count) ShardFinished() bool {
	return false
}

func (j *Count) Append(_ interface{}) error {
	*j++
	return nil
}

func (j Count) String() string {
	return fmt.Sprintf("records written = %d", j)
}

func (j Count) StartSnapshot(records <-chan interface{}, snapresp <-chan error) error {
	go func() {
		num := 0
		for _ = range records {
			num++
		}
		fmt.Printf("snapshotted %d records", num)
		snapresp <- nil
	}()
	return nil
}
