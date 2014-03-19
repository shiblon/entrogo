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
	"strconv"
	"strings"
	"testing"
	"time"

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

// ExampleTaskStore_MapReduce tests the taskstore by setting up a fake pipeline and
// working it for a while, just to make sure that things don't really hang up.
func ExampleTaskStore_MapReduce() {
	// We test the taskstore by creating a simple mapreduce pipeline.
	// This produces a word frequency histogram for the text below by doing the
	// following:
	//
	// - The lines of text create tasks, one for each line.
	// - Map goroutines consume those tasks, producing reduce groups.
	// - When all mapping is finished, one reduce task per group is created.
	// - Reduce goroutines consume reduce tasks, indicating which group to pull tasks from.
	// - They hold onto their reduce token, and so long as they own it, they
	// 	 perform reduce tasks and, when finished, push results into the result group.
	// - The results are finally read into a histogram.

	type Data struct {
		Key   string
		Count int
	}

	lines := []string{
		"The fundamental approach to parallel computing in a mapreduce environment",
		"is to think of computation as a multi-stage process, with a communication",
		"step in the middle. Input data is consumed in chunks by mappers. These",
		"mappers produce key/value pairs from their own data, and they are designed",
		"to do their work in isolation. Their computation does not depend on the",
		"computation of any of their peers. These key/value outputs are then grouped",
		"by key, and the reduce phase begins. All values corresponding to a",
		"particular key are processed together, producing a single summary output",
		"for that key. One example of a mapreduce is word counting. The input",
		"is a set of documents, the mappers produce word/count pairs, and the",
		"reducers compute the sum of all counts for each word, producing a word",
		"frequency histogram.",
	}

	mainID := rand.Int31()

	// Create a taskstore backed by a fake in-memory journal.
	fs := journal.NewMemFS("/myfs")
	jr, err := journal.NewDiskLogInjectFS("/myfs", fs)
	if err != nil {
		panic(fmt.Sprintf("failed to create journal: %v", err))
	}
	store := NewStrict(jr)

	// Insert text line tasks one at a time.
	toAdd := make([]*Task, len(lines))
	for i, line := range lines {
		toAdd[i] = NewTask("map", line)
	}
	_, err = store.Update(mainID, toAdd, nil, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("could not create task: %v", err))
	}

	// Expect to see one completion per line.
	mapperDone := make(chan bool, len(lines))

	// Start mapper workers.
	for i := 0; i < 3; i++ {
		go func() {
			mapperID := rand.Int31()
			for {
				// Get a task for ten seconds.
				claimed, err := store.Claim(mapperID, []string{"map"}, 10000)
				if err != nil {
					panic(fmt.Sprintf("error retrieving tasks: %v", err))
				}
				if claimed == nil {
					time.Sleep(5 * time.Second)
					continue
				}
				maptask := claimed[0]
				// Now we have a map task. Split the data into words and emit reduce tasks for them.
				// The data is just a line from the text file.
				words := strings.Split(maptask.Data, " ")
				wm := make(map[string]int)
				for _, word := range words {
					word = strings.ToLower(word)
					word = strings.TrimSuffix(word, ".")
					word = strings.TrimSuffix(word, ",")
					wm[strings.ToLower(word)]++
				}
				// One task per word, each in its own group (the word's group)
				reduceTasks := make([]*Task, 0)
				for word, count := range wm {
					group := fmt.Sprintf("reduceword %s", word)
					reduceTasks = append(reduceTasks, NewTask(group, fmt.Sprintf("%d", count)))
				}
				delTasks := []int64{maptask.ID}
				_, err = store.Update(mapperID, reduceTasks, nil, delTasks, nil)
				if err != nil {
					panic(fmt.Sprintf("mapper failed: %v", err))
				}

				mapperDone <- true
			}
		}()
	}

	for i := 0; i < len(lines); i++ {
		<-mapperDone
	}

	// Now do reductions. To do this we list all of the reduceword groups and
	// create a task for each, then we start the reducers.
	//
	// Note that there are almost certainly better ways to do this, but this is
	// simple and good for demonstration purposes.
	//
	// Why create a task? Because tasks, unlike groups, can be exclusively
	// owned and used as dependencies in updates.
	groups := store.Groups()
	reduceTasks := make([]*Task, 0)
	for _, g := range groups {
		if !strings.HasPrefix(g, "reduceword ") {
			continue
		}
		// Add the group name as a reduce task. A reducer will pick it up and
		// consume all tasks in the group.
		reduceTasks = append(reduceTasks, NewTask("reduce", g))
	}
	_, err = store.Update(mainID, reduceTasks, nil, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create reduce tasks: %v", err))
	}

	reducerDone := make(chan bool, len(reduceTasks))

	// Finally start the reducers.
	for i := 0; i < 3; i++ {
		go func() {
			reducerID := rand.Int31()
			for {
				claimed, err := store.Claim(reducerID, []string{"reduce"}, 30000)
				if err != nil {
					panic(fmt.Sprintf("failed to get reduce task: %v", err))
				}
				if claimed == nil {
					time.Sleep(5 * time.Second)
					continue
				}
				grouptask := claimed[0]
				word := strings.SplitN(grouptask.Data, " ", 2)[1]

				// No need to claim all of these tasks, just list them - the
				// main task is enough for claims, since we'll depend on it
				// before deleting these guys.
				tasks := store.ListGroup(grouptask.Data, 0, true)
				delTasks := make([]int64, len(tasks) + 1)
				sum := 0
				for i, task := range tasks {
					delTasks[i] = task.ID
					val, err := strconv.Atoi(task.Data)
					if err != nil {
						fmt.Printf("oops - weird value in task: %v\n", task)
						continue
					}
					sum += val
				}
				delTasks[len(delTasks)-1] = grouptask.ID
				outputTask := NewTask("output", fmt.Sprintf("%04d %s", sum, word))

				// Now we delete all of the reduce tasks, including the one
				// that we own that points to the group, and add an output
				// task.
				_, err = store.Update(reducerID, []*Task{outputTask}, nil, delTasks, nil)
				if err != nil {
					panic(fmt.Sprintf("failed to delete reduce tasks and create output: %v", err))
				}

				reducerDone <- true
			}
		}()
	}

	for i := 0; i < len(reduceTasks); i++ {
		<-reducerDone
	}

	// And now we have the finished output in the task store.
	outputTasks := store.ListGroup("output", 0, false)
	freqs := make([]string, len(outputTasks))
	for i, t := range outputTasks {
		freqs[i] = t.Data
	}
	sort.Sort(sort.Reverse(sort.StringSlice(freqs)))

	max := 10
	if max > len(freqs) {
		max = len(freqs)
	}
	for i := 0; i < max; i++ {
		fmt.Println(freqs[i])
	}

	// Output:
	//
	// 0008 the
	// 0008 a
	// 0006 of
	// 0004 to
	// 0004 their
	// 0004 is
	// 0004 in
	// 0003 word
	// 0003 mappers
	// 0003 key
}
