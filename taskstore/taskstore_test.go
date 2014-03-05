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
	"fmt"
	"math/rand"

	"code.google.com/p/entrogo/taskstore/journal"
)

func ExampleTaskStore_Add() {
	jr := journal.NewCount()
	ts := NewStrict(jr)
	adds := []*Task{
		NewTask(13, "mygroup"),
	}
	owner := rand.Int31()
	_, err := ts.Update(owner, adds, nil, nil, nil)
	fmt.Println(ts)
	fmt.Println("Err:", err)
	fmt.Printf("Journal: %v", jr)

	// Output:
	//
	// TaskStore:
	//   groups:
	//     "mygroup"
	//   snapshotting: false
	//   num tasks: 1
	//   last task id: 1
	// Err: <nil>
	// Journal: records written = 1
}
