// Copyright 2014 Chris Monson <shiblon@gmail.com>
//
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

package taskstore

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

type Journaler interface {
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
	Snapshot(records <-chan interface{}) error
}

type BytesJournaler struct {
	enc  *gob.Encoder
	buff *bytes.Buffer
}

func NewBytesJournaler() *BytesJournaler {
	j := &BytesJournaler{
		buff: new(bytes.Buffer),
	}
	j.enc = gob.NewEncoder(j.buff)
	return j
}

func (j BytesJournaler) ShardFinished() bool {
	return false
}

func (j BytesJournaler) AppendRecord(rec interface{}) error {
	return j.enc.Encode(rec)
}

func (j BytesJournaler) Bytes() []byte {
	return j.buff.Bytes()
}

func (j BytesJournaler) String() string {
	return j.buff.String()
}

func (j *BytesJournaler) Snapshot(records <-chan interface{}) error {
	return nil
}

type CountJournaler int64

func NewCountJournaler() *CountJournaler {
	return new(CountJournaler)
}

func (j CountJournaler) ShardFinished() bool {
	return false
}

func (j *CountJournaler) AppendRecord(_ interface{}) error {
	*j++
	return nil
}

func (j CountJournaler) String() string {
	return fmt.Sprintf("CountJournaler records written = %d", j)
}

func (j CountJournaler) Snapshot(records <-chan interface{}) error {
	num := 0
	for _ = range records {
		num++
	}
	fmt.Printf("CountJournaler snapshotted %d records", num)
	return nil
}
