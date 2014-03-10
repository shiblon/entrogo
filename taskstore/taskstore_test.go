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
	"log"
	"testing"

	"code.google.com/p/entrogo/taskstore/journal"
)

func TestTaskStore_Add(t *testing.T) {
	fs := journal.NewMemFS("/myfs")
	jr, err := journal.NewDiskLogInjectFS("/myfs", fs)
	if err != nil {
		t.Fatalf("failed to create journal: %v", err)
	}
	store := NewStrict(jr)

	var ownerID int32 = 11

	tasks := []*Task{
		NewTask("g1", "hello there"),
		NewTask("g1", "hi"),
		NewTask("g2", "10"),
		NewTask("g2", "5"),
		NewTask("g3", "-"),
		NewTask("g3", "_"),
	}

	newtasks, err := store.Update(ownerID, tasks, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to add new tasks: %v", err)
	}

	// TODO: what do we check for here?
	log.Println(newtasks)
	log.Println(fs)
}
