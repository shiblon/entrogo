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
	"os"
	"sort"
	"strings"
	"testing"

	"code.google.com/p/entrogo/taskstore/journal"
)

func ExampleTaskStore() {
	// The task store is only the storage portion of a task queue. If you wish
	// to implement a service, you can easily do so using the primitives
	// provided. This example should give an idea of how you might do that.

	// To create a task store, specify a journal implementation. This
	// particular implementation will attempt to lock a file in the specified
	// directory and will panic if it cannot. It is not a guarantee against
	// simultaneous unsafe access in a networked environment, however. For that
	// a consensus protocol or something similar is recommended, implemented at
	// the service level.
	jr, err := journal.NewDiskLog("/tmp/taskjournal")
	if err != nil {
		panic(fmt.Sprintf("could not create journal: %v", err))
	}

	// Then create the task store itself. You can create a "strict" store,
	// which requires that all transactions be flushed to the journal before
	// being committed to memory (and results returned), or "opportunistic",
	// which commits to memory and returns while letting journaling happen in
	// the background. If task execution is idempotent and it is always obvious
	// when to retry, you can get a speed benefit from opportunistic
	// journaling.
	store := NewStrict(jr)

	// To put a task into the store, call Update with the "add" parameter:
	add := []*Task{
		NewTask("groupname", "task info, any string"),
	}

	// Every user of the task store needs a unique "OwnerID". When implementing
	// this as a service, the client library would likely assign this at
	// startup, so each process gets its own (and cannot change it). This is
	// one example of how to create an Owner ID.
	clientID := int32(rand.Int() ^ os.Getpid())

	// Request an update. Here you can add, modify, and delete multiple tasks
	// simultaneously. You can also specify a set of task IDs that must be
	// present (but will not be modified) for this operation to succeed.
	results, err := store.Update(clientID, add, nil, nil, nil)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// If successful, "results" will contain all of the newly-created tasks.
	// Note that even a task modification is relaly a task creation: it deletes
	// the old task and creates a new task with a new ID. IDs are guarnteed to
	// increase monotonically.
	fmt.Println(results)
}

func TestTaskStore_Update(t *testing.T) {
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

	added, err := store.Update(ownerID, tasks, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to add new tasks: %v", err)
	}

	now := NowMillis()

	// Ensure that the tasks are exactly what we added, but with id values, etc.
	for i, task := range tasks {
		nt := added[i]
		if nt.ID <= 0 {
			t.Errorf("new task should have non-zero assigned ID, has %d", nt.ID)
		}
		if task.Group != nt.Group {
			t.Errorf("expected task group %q, got %q", task.Group, nt.Group)
		}
		if nt.OwnerID != ownerID {
			t.Errorf("expected owner ID %d, got %d", ownerID, nt.OwnerID)
		}
		if nt.AvailableTime > now {
			t.Errorf("new task has assigned available time in the future. expected t <= %d, got %d", now, nt.AvailableTime)
		} else if nt.AvailableTime <= 0 {
			t.Errorf("expected valid available time, got %d", nt.AvailableTime)
		}
		if nt.Data != task.Data {
			t.Errorf("expected task data to be %q, got %q", task.Data, nt.Data)
		}
	}

	groups := store.Groups()
	sort.Strings(groups)

	wantedGroups := []string{"g1", "g2", "g3"}

	if !eqStrings(wantedGroups, groups) {
		t.Fatalf("expected groups %v, got %v", wantedGroups, groups)
	}

	g1Tasks := store.ListGroup("g1", -1, true)
	if len(g1Tasks) != 2 {
		t.Errorf("g1 should have 2 tasks, has %v", g1Tasks)
	}

	// Try getting a non-existent group. Should come back nil.
	nongroupTasks := store.ListGroup("nongroup", -1, true)
	if nongroupTasks != nil {
		t.Errorf("nongroup should come back nil (no tasks, but not an error), came back %v", nongroupTasks)
	}

	// Now claim a task by setting its AvailableTime in the future by some number of milliseconds.
	t0 := added[0].Copy()
	t0.AvailableTime += 60 * 1000 // 1 minute into the future, this will expire
	updated, err := store.Update(ownerID, nil, []*Task{t0}, nil, nil)
	if err != nil {
		t.Fatalf("failed to update task %v: %v", added[0], err)
	}
	if updated[0].ID <= added[0].ID {
		t.Errorf("expected updated task to have ID > original, but got %d <= %d", updated[0].ID, added[0].ID)
	}
	if updated[0].Group != added[0].Group {
		t.Errorf("expected updated task to have group %q, got %q", added[0].Group, updated[0].Group)
	}
	if updated[0].OwnerID != ownerID {
		t.Errorf("expected updated task to have owner ID %d, got %d", ownerID, updated[0].OwnerID)
	}
	if updated[0].AvailableTime-added[0].AvailableTime != 60000 {
		t.Errorf("expected updated task to expire 1 minute later than before, but got a difference of %d", updated[0].AvailableTime-added[0].AvailableTime)
	}
	// Task is now owned, so it should not come back if we disallow owned tasks in a group listing.
	g1Available := store.ListGroup("g1", 0, false)
	if len(g1Available) > 1 {
		t.Errorf("expected 1 unowned task in g1, got %d", len(g1Available))
	}
	if g1Available[0].ID == updated[0].ID {
		t.Errorf("expected to get a task ID != %d (an unowned task), but got it anyway", updated[0].ID)
	}

	// This owner should be able to update its own future task.
	t0 = updated[0].Copy()
	t0.AvailableTime += 1000
	updated2, err := store.Update(ownerID, nil, []*Task{t0}, nil, nil)
	if err != nil {
		t.Fatalf("couldn't update future task: %v", err)
	}
	if updated2[0].AvailableTime-updated[0].AvailableTime != 1000 {
		t.Errorf("expected 1-second increase in available time, got difference of %d milliseconds", updated2[0].AvailableTime-updated[0].AvailableTime)
	}

	// But another owner should not be able to touch it.
	t0 = updated2[0].Copy()
	t0.AvailableTime += 2000
	_, err = store.Update(ownerID+1, nil, []*Task{t0}, nil, nil)
	if err == nil {
		t.Fatalf("owner %d should not succeed in updated task owned by %d", ownerID+1, ownerID)
	}
	uerr, ok := err.(UpdateError)
	if !ok {
		t.Fatalf("unexpected error type, could not convert to UpdateError: %#v", err)
	}
	if len(uerr.Errors) != 1 {
		t.Errorf("expected 1 error in UpdateError list, got %d", len(uerr.Errors))
	}
	if !strings.Contains(uerr.Errors[0].Error(), fmt.Sprintf("owned by %d, cannot be changed by %d", ownerID, ownerID+1)) {
		t.Errorf("expected ownership error, got %v", uerr.Errors)
	}

	// Now try to update something that depends on an old task (our original
	// task, which has now been updated and is therefore no longer present).
	_, err = store.Update(ownerID, nil, []*Task{t0}, nil, []int64{tasks[0].ID})
	if err == nil {
		t.Fatalf("expected updated dependent on %d to fail, as that task should not be around", tasks[0].ID)
	}
	if !strings.Contains(err.(UpdateError).Errors[0].Error(), "unmet dependency:") {
		t.Fatalf("expected unmet dependency error, got %v", err.(UpdateError).Errors)
	}

	// Try updating a task that we already updated.
	_, err = store.Update(ownerID, nil, []*Task{updated[0]}, nil, nil)
	if err == nil {
		t.Fatalf("expected to get an error when updating a task that was already updated")
	}
	if !strings.Contains(err.(UpdateError).Errors[0].Error(), "not found") {
		t.Fatalf("expected task not found error, got %v", err.(UpdateError).Errors)
	}

	// And now try deleting a task.
	updated3, err := store.Update(ownerID, nil, nil, []int64{added[2].ID}, nil)
	if err != nil {
		t.Fatalf("deletion of task %v failed: %v", added[2], err)
	}
	if len(updated3) != 1 {
		t.Fatalf("expected 1 update with a task deletion, got %v", updated3)
	}

	all := make(map[int64]*Task)
	for _, g := range store.Groups() {
		for _, t := range store.ListGroup(g, 0, true) {
			all[t.ID] = t
		}
	}

	expectedData := map[int64]string{
		2: "hi",
		4: "5",
		5: "-",
		6: "_",
		8: "hello there", // last to be updated, so it moved to the end
	}

	for id, data := range expectedData {
		if all[id].Data != data {
			t.Errorf("full dump: expected %q, got %q", data, all[id].Data)
		}
	}
}

func eqStrings(l1, l2 []string) bool {
	if len(l1) != len(l2) {
		return false
	}
	for i := range l1 {
		if l1[i] != l2[i] {
			return false
		}
	}
	return true
}
