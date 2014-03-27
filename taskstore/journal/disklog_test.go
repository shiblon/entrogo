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

package journal

import (
	"fmt"
	"io"
	"path/filepath"
	"testing"
)

func ExampleDiskLog() {
	// Open up the log in directory "/tmp/disklog". Will create an error if it does not exist.
	fs := NewMemFS("/tmp/disklog")
	journal, err := OpenDiskLogInjectFS("/tmp/disklog", fs)
	if err != nil {
		fmt.Printf("Failed to open /tmp/disklog: %v\n", err)
		return
	}

	// Data type can be anything. Here we're adding integers one at a time. We
	// could also add the entire list at once, since it just gets gob-encoded.
	data := []int{2, 3, 5, 7, 11, 13}
	for _, d := range data {
		if err := journal.Append(d); err != nil {
			fmt.Printf("Failed to append %v: %v\n", d, err)
		}
	}
	// We didn't write enough to trigger a rotation, so everything should be in
	// the current journal. Let's see if we get it back.
	decoder, err := journal.JournalDecoder()
	if err != nil {
		fmt.Printf("error getting decoder: %v\n", err)
		return
	}
	vals := make([]int, 0)
	val := -1
	for {
		err := decoder.Decode(&val)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error:", vals)
			fmt.Printf("failed to decode next item in journal: %v\n", err)
			return
		}
		vals = append(vals, val)
	}
	fmt.Println("Success", vals)

	// Output:
	//
	// Success [2 3 5 7 11 13]
}

func TestDiskLog_Rotate(t *testing.T) {
	fs := NewMemFS("/tmp/disklog")
	journal, err := OpenDiskLogInjectFS("/tmp/disklog", fs)
	if err != nil {
		t.Fatalf("failed to create memfs disklog: %v", err)
	}

	// Add data, rotate, then add more data

	beforedata := []int{2, 3, 5, 7, 11}
	afterdata := []int{13, 17, 23}
	for _, d := range beforedata {
		if err := journal.Append(d); err != nil {
			t.Fatalf("failed to append data: %v", err)
		}
	}
	journal.Rotate()
	for _, d := range afterdata {
		if err := journal.Append(d); err != nil {
			t.Fatalf("failed to append data: %v", err)
		}
	}

	// Pull all of the data out. We should get all of it in order.
	decoder, err := journal.JournalDecoder()
	if err != nil {
		t.Fatalf("failed to create a decoder: %v", err)
	}
	var val int
	vals := make([]int, 0)
	for {
		err := decoder.Decode(&val)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		vals = append(vals, val)
	}

	alldata := append(beforedata, afterdata...)
	good := true
	if len(alldata) != len(vals) {
		good = false
	} else {
		for i, d := range vals {
			if d != alldata[i] {
				good = false
				break
			}
		}
	}
	if !good {
		t.Errorf("expected\n%v\ngot\n%v", alldata, vals)
	}

	// finally, check that we have two different files, one open and one frozen.
	working, err := fs.FindMatching("/tmp/disklog/*.log.working")
	if err != nil {
		t.Fatalf("error getting working files: %v", err)
	}
	frozen, err := fs.FindMatching("/tmp/disklog/*.log")
	if err != nil {
		t.Fatalf("error getting frozen files: %v", err)
	}

	if len(working) != 1 {
		t.Fatalf("expected %d working file(s), found %d", 1, len(working))
	}
	if len(frozen) != 1 {
		t.Fatalf("expected %d frozen file(s), found %d", 1, len(frozen))
	}

	workbase := filepath.Base(working[0])
	frozenbase := filepath.Base(frozen[0])

	workts, err := TSFromName(workbase)
	if err != nil {
		t.Fatalf("can't get timestamp from working name %q: %v", workbase, err)
	}
	frozents, err := TSFromName(frozenbase)
	if err != nil {
		t.Fatalf("can't get timestamp from frozen name %q: %v", frozenbase, err)
	}

	if workts <= frozents {
		t.Fatalf(
			"working logs should always be newer than frozen logs. "+
				"Got working=%d, frozen=%d from\n%q\n%q", workts, frozents, workbase, frozenbase)
	}
}

func TestDiskLog_Decode_Corrupt(t *testing.T) {
	// Open up the log in directory "/tmp/disklog". Will create an error if it does not exist.
	fs := NewMemFS("/tmp/disklog")
	journal, err := OpenDiskLogInjectFS("/tmp/disklog", fs)
	if err != nil {
		t.Fatalf("failed to open /tmp/disklog: %v\n", err)
	}

	data := []int{2, 3, 5, 7, 11, 13}
	for _, d := range data {
		if err := journal.Append(d); err != nil {
			t.Fatalf("failed to append %v: %v\n", d, err)
		}
	}

	working, err := fs.FindMatching("/tmp/disklog/*.log.working")
	if err != nil {
		t.Fatalf("could not match: %v", err)
	}
	if len(working) == 0 {
		t.Fatalf("no working files found")
	}

	// Write bogus data at the end, but not 0 padding:
	fs.files[working[0]].Write([]byte{2})

	// And try to decode, including the last bogus record.
	// It should be ignored by the decoder.
	decoder, err := journal.JournalDecoder()
	if err != nil {
		t.Fatalf("error getting decoder: %v\n", err)
		return
	}
	vals := make([]int, 0)
	val := -1
	for {
		err := decoder.Decode(&val)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Error("Error:", vals)
			t.Fatalf("failed to decode next item in journal: %v\n", err)
		}
		vals = append(vals, val)
	}

	// Then ensure that we still have all the data AND that we got the unexpected EOF.
	for i, d := range vals {
		if d != data[i] {
			t.Fatalf("Expected %v, got %v", data, vals)
		}
	}
	// TODO: ensure that the log receives an unexpected EOF warning message.
}

func TestDiskLog_Snapshot(t *testing.T) {
	type dtype struct {
		S string
		I int
	}
	data := []dtype{
		{"blah", 3},
		{"hi", 1},
		{"hello", 2},
	}

	fs := NewMemFS("/myfs")
	journal, err := OpenDiskLogInjectFS("/myfs", fs)
	if err != nil {
		t.Fatalf("failed to create journal: %v", err)
	}

	// We don't have to append to the journal to create a snapshot, so we don't
	// bother. Just start a snapshot and provide the right data.
	recs := make(chan interface{}, 1)
	resp := make(chan error, 1)
	go func() {
		defer close(recs)
		for _, d := range data {
			recs <- d
		}
	}()
	if err := journal.StartSnapshot(recs, resp); err != nil {
		t.Fatalf("error starting snapshot: %v", err)
	}

	if err := <-resp; err != nil {
		t.Fatalf("error in response to snapshot request: %v", err)
	}

	// Now the snapshot is complete. Ensure that it is there.
	names, err := fs.FindMatching("/myfs/*.*.snapshot")
	if len(names) != 1 {
		t.Fatalf("expected 1 snapshot, found %d: %v", len(names), names)
	}

	// And replay the data.
	decoder, err := journal.SnapshotDecoder()
	if err != nil {
		t.Fatalf("error getting snapshot decoder: %v", err)
	}

	replayed := make([]dtype, 0, len(data))
	var val dtype
	for {
		err := decoder.Decode(&val)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error decoding value in snapshot: %v", err)
		}
		replayed = append(replayed, val)
	}

	// And finally test that we have equality.
	for i, d := range data {
		if d != replayed[i] {
			t.Fatalf("expected %v, got %v on replay", data, replayed)
		}
	}
}
