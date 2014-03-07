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
	"testing"
)

// To corrupt the end of the file:
// dl.journalFile.Write([]byte{2})


func ExampleDiskLog() {
	// Open up the log in directory "/tmp/disklog". Will create an error if it does not exist.
	fsimpl := NewMemFS([]string{"/tmp/disklog"})
	dl, err := NewDiskLogInjectOS("/tmp/disklog", fsimpl)
	if err != nil {
		fmt.Printf("Failed to open /tmp/disklog: %v\n", err)
		return
	}

	// Data type can be anything. Here we're adding integers one at a time. We
	// could also add the entire list at once, since it just gets gob-encoded.
	data := []int{2, 3, 5, 7, 11, 13}
	for _, d := range data {
		if err := dl.Append(d); err != nil {
			fmt.Printf("Failed to append %v: %v\n", d, err)
		}
	}
	// We didn't write enough to trigger a rotation, so everything should be in
	// the current journal. Let's see if we get it back.
	decoder, err := dl.JournalDecoder()
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

func TestNewDiskLog(*testing.T) {
	// dl, err := NewDiskLog("/non-existent/directory")
}
