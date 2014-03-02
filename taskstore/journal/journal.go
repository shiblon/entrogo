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
	// ShardFinished is expected to return true precisely when the next journal
	// write will trigger a rotation, i.e., the current shard will be closed and
	// made immutable, and a new shard will be started on the next write.
	ShardFinished() bool

	// AppendRecord appends a serialized version of the interface to the
	// current journal shard.
	AppendRecord(interface{}) error

	// Snapshot is given a data channel from which it is expected to consume
	// all values until closed. If it terminates early, it sends a non-nil
	// error back. When complete with no errors, the snapshot has been
	// successfully processed. Whether the current shard is full or not, this
	// function should immediately trigger a shard rotation so that subsequent
	// calls to AppendRecord go to a new shard.
	Snapshot(records <-chan interface{}, resp <-chan error) error
}

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

func (j Bytes) AppendRecord(rec interface{}) error {
	return j.enc.Encode(rec)
}

func (j Bytes) Bytes() []byte {
	return j.buff.Bytes()
}

func (j Bytes) String() string {
	return j.buff.String()
}

func (j *Bytes) Snapshot(records <-chan interface{}, snapresp <-chan error) error {
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

func (j *Count) AppendRecord(_ interface{}) error {
	*j++
	return nil
}

func (j Count) String() string {
	return fmt.Sprintf("records written = %d", j)
}

func (j Count) Snapshot(records <-chan interface{}, snapresp <-chan error) error {
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
