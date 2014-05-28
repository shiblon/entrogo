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

	// To create a task store, specify a journal implementation. A real
	// implementation should use a lock of some kind to guarantee exclusivity.
	jr, err := journal.OpenDiskLog("/tmp/taskjournal")
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
	store, err := OpenStrict(jr)
	if err != nil {
		fmt.Print("error opening taskstore: %v\n", err)
		return
	}
	defer store.Close()

	// To put a task into the store, call Update with the "add" parameter:
	add := []*Task{
		NewTask("groupname", []byte("task info, any string")),
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
	jr, err := journal.OpenDiskLogInjectFS("/myfs", fs)
	if err != nil {
		t.Fatalf("failed to create journal: %v", err)
	}
	store, err := OpenStrict(jr)
	if err != nil {
		t.Fatalf("error opening taskstore: %v\n", err)
	}
	if !store.IsOpen() {
		t.Fatalf("task store not open after call to OpenStrict")
	}
	defer store.Close()

	var ownerID int32 = 11

	tasks := []*Task{
		NewTask("g1", []byte("hello there")),
		NewTask("g1", []byte("hi")),
		NewTask("g2", []byte("10")),
		NewTask("g2", []byte("5")),
		NewTask("g3", []byte("-")),
		NewTask("g3", []byte("_")),
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
		if nt.AT > now {
			t.Errorf("new task has assigned available time in the future. expected t <= %d, got %d", now, nt.AT)
		} else if nt.AT <= 0 {
			t.Errorf("expected valid available time, got %d", nt.AT)
		}
		if string(nt.Data) != string(task.Data) {
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

	// Now claim a task by setting its AT in the future by some number of milliseconds.
	t0 := added[0].Copy()
	t0.AT += 60 * 1000 // 1 minute into the future, this will expire
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
	if updated[0].AT-added[0].AT != 60000 {
		t.Errorf("expected updated task to expire 1 minute later than before, but got a difference of %d", updated[0].AT-added[0].AT)
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
	t0.AT += 1000
	updated2, err := store.Update(ownerID, nil, []*Task{t0}, nil, nil)
	if err != nil {
		t.Fatalf("couldn't update future task: %v", err)
	}
	if updated2[0].AT-updated[0].AT != 1000 {
		t.Errorf("expected 1-second increase in available time, got difference of %d milliseconds", updated2[0].AT-updated[0].AT)
	}

	// But another owner should not be able to touch it.
	t0 = updated2[0].Copy()
	t0.AT += 2000
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
	if len(updated3) != 0 {
		t.Fatalf("expected 0 updated tasks, got %v", updated3)
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
		if string(all[id].Data) != string(data) {
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

// ExampleTaskStore_tasks demonstrates the use of getting tasks by id.
func ExampleTaskStore_tasks() {

}

// ExampleTaskStore_mapReduce tests the taskstore by setting up a fake pipeline and
// working it for a while, just to make sure that things don't really hang up.
func ExampleTaskStore_mapReduce() {
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

	numMappers := 3
	numReducers := 3
	maxSleepMillis := 500

	mainID := rand.Int31()

	// Create a taskstore backed by a fake in-memory journal.
	fs := journal.NewMemFS("/myfs")
	jr, err := journal.OpenDiskLogInjectFS("/myfs", fs)
	if err != nil {
		panic(fmt.Sprintf("failed to create journal: %v", err))
	}
	store, err := OpenStrict(jr)
	if err != nil {
		fmt.Printf("error opening task store: %v\n", err)
		return
	}
	defer store.Close()

	// And add all of the input lines.
	toAdd := make([]*Task, len(lines))
	for i, line := range lines {
		toAdd[i] = NewTask("map", []byte(line))
	}

	// Do the actual update.
	_, err = store.Update(mainID, toAdd, nil, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("could not create task: %v", err))
	}

	// Start mapper workers.
	for i := 0; i < numMappers; i++ {
		go func() {
			mapperID := rand.Int31()
			for {
				// Get a task for ten seconds.
				maptask, err := store.Claim(mapperID, "map", 10000, nil)
				if err != nil {
					panic(fmt.Sprintf("error retrieving tasks: %v", err))
				}
				if maptask == nil {
					time.Sleep(time.Duration(maxSleepMillis) * time.Millisecond)
					continue
				}
				// Now we have a map task. Split the data into words and emit reduce tasks for them.
				// The data is just a line from the text file.
				words := strings.Split(string(maptask.Data), " ")
				wm := make(map[string]int)
				for _, word := range words {
					word = strings.ToLower(word)
					word = strings.TrimSuffix(word, ".")
					word = strings.TrimSuffix(word, ",")
					wm[strings.ToLower(word)]++
				}
				// One task per word, each in its own group (the word's group)
				// This could just as easily be something in the filesystem,
				// and the reduce tasks would just point to them, but we're
				// using the task store because our data is small and because
				// we can.
				reduceTasks := make([]*Task, 0)
				for word, count := range wm {
					group := fmt.Sprintf("reduceword %s", word)
					reduceTasks = append(reduceTasks, NewTask(group, []byte(fmt.Sprintf("%d", count))))
				}
				delTasks := []int64{maptask.ID}
				_, err = store.Update(mapperID, reduceTasks, nil, delTasks, nil)
				if err != nil {
					panic(fmt.Sprintf("mapper failed: %v", err))
				}
			}
		}()
	}

	// Just wait for all map tasks to be deleted.
	for {
		tasks := store.ListGroup("map", 1, true)
		if len(tasks) == 0 {
			break
		}
		time.Sleep(time.Duration(rand.Intn(maxSleepMillis)+1) * time.Millisecond)
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
	reduceTasks := make([]*Task, 0, len(groups))
	for _, g := range groups {
		if !strings.HasPrefix(g, "reduceword ") {
			continue
		}
		// Add the group name as a reduce task. A reducer will pick it up and
		// consume all tasks in the group.
		reduceTasks = append(reduceTasks, NewTask("reduce", []byte(g)))
	}

	_, err = store.Update(mainID, reduceTasks, nil, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create reduce tasks: %v", err))
	}

	// Finally start the reducers.
	for i := 0; i < numReducers; i++ {
		go func() {
			reducerID := rand.Int31()
			for {
				grouptask, err := store.Claim(reducerID, "reduce", 30000, nil)
				if err != nil {
					panic(fmt.Sprintf("failed to get reduce task: %v", err))
				}
				if grouptask == nil {
					time.Sleep(time.Duration(maxSleepMillis) * time.Millisecond)
					continue
				}
				gtdata := string(grouptask.Data)
				word := strings.SplitN(gtdata, " ", 2)[1]

				// No need to claim all of these tasks, just list them - the
				// main task is enough for claims, since we'll depend on it
				// before deleting these guys.
				tasks := store.ListGroup(gtdata, 0, true)
				delTasks := make([]int64, len(tasks)+1)
				sum := 0
				for i, task := range tasks {
					delTasks[i] = task.ID
					val, err := strconv.Atoi(string(task.Data))
					if err != nil {
						fmt.Printf("oops - weird value in task: %v\n", task)
						continue
					}
					sum += val
				}
				delTasks[len(delTasks)-1] = grouptask.ID
				outputTask := NewTask("output", []byte(fmt.Sprintf("%04d %s", sum, word)))

				// Now we delete all of the reduce tasks, including the one
				// that we own that points to the group, and add an output
				// task.
				_, err = store.Update(reducerID, []*Task{outputTask}, nil, delTasks, nil)
				if err != nil {
					panic(fmt.Sprintf("failed to delete reduce tasks and create output: %v", err))
				}

				// No need to signal anything - we just deleted the reduce
				// task. The main process can look for no tasks remaining.
			}
		}()
	}

	// Just look for all reduce tasks to be finished.
	for {
		tasks := store.ListGroup("reduce", 1, true)
		if len(tasks) == 0 {
			break
		}
		time.Sleep(time.Duration(rand.Intn(maxSleepMillis)+1) * time.Millisecond)
	}

	// And now we have the finished output in the task store.
	outputTasks := store.ListGroup("output", 0, false)
	freqs := make([]string, len(outputTasks))
	for i, t := range outputTasks {
		freqs[i] = string(t.Data)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(freqs)))

	for i, f := range freqs {
		if i >= 10 {
			break
		}
		fmt.Println(f)
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

func TestTaskStore_Fuzz(t *testing.T) {
	// TODO:
	//
	// Make a list of things to do (managed as conditions). Try to cover all of the operations.
	// The main difficulties are
	// - Open
	// - Close
	// - Claim
	// 	 - Make sure to test claiming existing tasks and non-existing tasks.
	// - Update
	// 	 - Make sure to add,
	// 	 - delete (existing and non-existing),
	// 	 - update (existing and non-existing),
	// 	 - depend (existing and non-existing),
	// 	 The trick here is going to be making sure that we have some operations
	// 	 that actually succeed. So we'll need to randomly create successful
	// 	 operations and then randomly mutate them to fail once in a while for
	// 	 various reasons.
	// The rest are read-only operations. We want to fuzz mutations (but we
	// also want to be sure that the read-only stuff actually works).
}

// Pre/Post conditions for various calls
type Condition interface {
	Pre() bool
	Call()
	Post() bool
}

// A ClaimCond embodies the pre and post conditions for calling TaskStore.Claim.
type ClaimCond struct {
	store *TaskStore

	preDepend []*Task

	argOwner    int32
	argGroup    string
	argDuration int64
	argDepend   []int64

	retTask *Task
	retErr  error

	preNow            int64
	preOpen           bool
	preUnownedInGroup int
}

func NewClaimCond(store *TaskStore, owner int32, group string, duration int64, depends []int64) *ClaimCond {
	return &ClaimCond{
		store:       store,
		argOwner:    owner,
		argGroup:    group,
		argDuration: duration,
		argDepend:   depends,
	}
}

func (c *ClaimCond) Pre() bool {
	c.preNow = NowMillis()
	c.preOpen = c.store.IsOpen()
	if c.preOpen {
		c.preUnownedInGroup = len(c.store.ListGroup(c.argGroup, 0, false))
		c.preDepend = c.store.Tasks(c.argDepend)
	}
	return true
}

func (c *ClaimCond) Call() {
	c.retTask, c.retErr = c.store.Claim(c.argOwner, c.argGroup, c.argDuration, c.argDepend)
}

func (c *ClaimCond) Post() bool {
	if !c.preOpen {
		if c.store.IsOpen() {
			fmt.Println("Claim Postcondition: store was not open, but is now")
			return false
		}
		if c.retErr == nil {
			fmt.Println("Claim Postcondition: no error returned when claiming from a closed store")
			return false
		}
		if numUnowned := len(c.store.ListGroup(c.argGroup, 0, false)); numUnowned != c.preUnownedInGroup {
			fmt.Println("Claim Postcondition: unowned tasks changed, even though the store is closed")
			return false
		}
		return true
	}

	// Check that failed dependencies cause a claim error
	var hasNil int64 = -1
	for i, task := range c.preDepend {
		if task == nil {
			hasNil = c.argDepend[i]
			break
		}
	}
	if hasNil >= 0 && c.retErr == nil {
		fmt.Printf("Claim Postcondition: dependency %d missing, but claim succeeded.\n", hasNil)
		return false
	}

	now := NowMillis()
	numUnowned := len(c.store.ListGroup(c.argGroup, 0, false))
	if c.preUnownedInGroup == 0 && numUnowned > 0 {
		fmt.Println("Claim Postcondition: no tasks to claim, magically produced a claimable task")
		return false
	}

	if c.retTask == nil || c.retErr != nil {
		fmt.Printf("Claim Postcondition: unowned tasks available, but none claimed: error %v\n", c.retErr)
		return false
	}

	if c.retTask.AT > now && numUnowned != (c.preUnownedInGroup-1) {
		fmt.Println("Claim Postcondition: unowned expected to change by -1, changed by %d",
			c.preUnownedInGroup-numUnowned)
		return false
	}

	if c.retTask.OwnerID != c.argOwner {
		fmt.Println("Claim Postcondition: owner not assigned properly to claimed task")
		return false
	}

	// TODO: check that the claimed task is actually one of the unowned tasks.

	return true
}

// A CloseCond embodies the pre and post conditions for the Close call.
type CloseCond struct {
	store   *TaskStore
	preOpen bool
	retErr  error
}

func NewCloseClond(store *TaskStore) *CloseCond {
	return &CloseCond{
		store: store,
	}
}

func (c *CloseCond) Pre() bool {
	c.preOpen = c.store.IsOpen()
	return true
}

func (c *CloseCond) Call() {
	c.retErr = c.store.Close()
}

func (c *CloseCond) Post() bool {
	postOpen := c.store.IsOpen()
	if !c.preOpen {
		if postOpen {
			fmt.Println("Close Postcondition: magically opened from closed state.")
			return false
		}
		if c.retErr == nil {
			fmt.Println("Close Postcondition: closed store, but Close did not return an error.")
			return false
		}
		return true
	}
	if c.retErr != nil {
		fmt.Printf("Close Postcondition: closed an open store, but got an error: %v\n", c.retErr)
		return false
	}
	if postOpen {
		fmt.Println("Close Postcondition: failed to close store; still open.")
		return false
	}
	return true
}

// A ListGroupCond embodies the pre and post conditions for listing tasks in a group.
type ListGroupCond struct {
	store         *TaskStore
	preOpen       bool
	preNow        int64
	argGroup      string
	argLimit      int
	argAllowOwned bool
	retTasks      []*Task
}

func NewListGroupCond(store *TaskStore, group string, limit int, allowOwned bool) *ListGroupCond {
	return &ListGroupCond{
		store:         store,
		argGroup:      group,
		argLimit:      limit,
		argAllowOwned: allowOwned,
	}
}

func (c *ListGroupCond) Pre() bool {
	c.preOpen = c.store.IsOpen()
	c.preNow = NowMillis()
	return true
}

func (c *ListGroupCond) Call() {
	c.retTasks = c.store.ListGroup(c.argGroup, c.argLimit, c.argAllowOwned)
}

func (c *ListGroupCond) Post() bool {
	if !c.preOpen {
		if c.retTasks != nil {
			fmt.Println("ListGroup Postcondition: returned non-nil tasks.")
			return false
		}
	}
	if c.argLimit <= 0 {
		return true // we can't really test this separately from itself.
	}
	if !c.argAllowOwned {
		for _, t := range c.retTasks {
			// Not allowing owned, but got owned tasks anyway.
			if t.AT > c.preNow {
				fmt.Println("ListGroup Postcondition: got owned tasks when not asking for them.")
				return false
			}
		}
	}
	if len(c.retTasks) > c.argLimit {
		fmt.Printf("ListGroup Postcondition: asked for max %d tasks, got more (%d).\n", c.argLimit, len(c.retTasks))
		return false
	}
	return true
}

// GroupsCond embodies the pre and post conditions for the store's Groups call.
type GroupsCond struct {
	store     *TaskStore
	preOpen   bool
	retGroups []string
}

func NewGroupsCond(store *TaskStore) *GroupsCond {
	return &GroupsCond{
		store: store,
	}
}

func (c *GroupsCond) Pre() bool {
	c.preOpen = c.store.IsOpen()
	return true
}

func (c *GroupsCond) Call() {
	c.retGroups = c.store.Groups()
}

func (c *GroupsCond) Post() bool {
	if !c.preOpen {
		if c.retGroups != nil {
			fmt.Println("Groups Postcondition: returned non-nil groups when closed.")
			return false
		}
		return true
	}
	if c.retGroups == nil {
		fmt.Println("Groups Postcondition: returned nil groups when open.")
		return false
	}
	return true
}

// NumTasksCond embodies the pre and post conditions for the NumTasks call.
type NumTasksCond struct {
	store   *TaskStore
	preOpen bool
	retNum  int32
}

func NewNumTasksCond(store *TaskStore) *NumTasksCond {
	return &NumTasksCond{
		store: store,
	}
}

func (c *NumTasksCond) Pre() bool {
	c.preOpen = c.store.IsOpen()
	return true
}

func (c *NumTasksCond) Call() {
	c.retNum = c.store.NumTasks()
}

func (c *NumTasksCond) Post() bool {
	if !c.preOpen {
		if c.retNum != 0 {
			fmt.Println("NumTasks Postcondition: non-zero tasks on closed store.")
			return false
		}
		return true
	}
	if c.retNum < 0 {
		fmt.Println("NumTasks Postcondition: negative task num returned.")
		return false
	}
	return true
}

// Tasks embodies the pre and post conditions for the Tasks call.
type TasksCond struct {
	store    *TaskStore
	preOpen  bool
	argIDs   []int64
	retTasks []*Task
}

func NewTasksCond(store *TaskStore, ids []int64) *TasksCond {
	return &TasksCond{
		store:  store,
		argIDs: ids,
	}
}

func (c *TasksCond) Pre() bool {
	c.preOpen = c.store.IsOpen()
	return true
}

func (c *TasksCond) Call() {
	c.retTasks = c.store.Tasks(c.argIDs)
}

func (c *TasksCond) Post() bool {
	if !c.preOpen {
		if c.retTasks != nil {
			fmt.Println("Tasks Postcondition: non-nil tasks on closed store.")
			return false
		}
		return true
	}
	if len(c.retTasks) > len(c.argIDs) {
		fmt.Println("Tasks Postcondition: more tasks returned than requested.")
		return false
	}
	idmap := make(map[int64]struct{})
	for _, id := range c.argIDs {
		idmap[id] = struct{}{}
	}
	for i, t := range c.retTasks {
		if t != nil {
			if _, ok := idmap[t.ID]; !ok {
				fmt.Printf("Tasks Postcondition: returned task %d not in requested ID list %d.\n", t.ID, c.argIDs)
				return false
			}
			if t.ID != c.argIDs[i] {
				fmt.Printf("Tasks Postcondition: returned task %d not expected task %d.\n", t.ID, c.argIDs[i])
				return false
			}
		}
		delete(idmap, c.argIDs[i])
	}
	if len(idmap) != 0 {
		fmt.Printf("Tasks Postcondition: not all tasks accounted for. missing %v.\n", idmap)
		return false
	}
	return true
}

// UpdateCond embodies the pre and post conditions for the Update call.
type UpdateCond struct {
	store     *TaskStore
	preOpen   bool
	preChange []*Task
	preDelete []*Task
	preDepend []*Task
	argOwner  int32
	argAdd    []*Task
	argChange []*Task
	argDelete []int64
	argDepend []int64
	retTasks  []*Task
	retErr    error
	now       int64
}

func NewUpdateCond(store *TaskStore, owner int32, add, change []*Task, del, dep []int64) *UpdateCond {
	return &UpdateCond{
		store:     store,
		argOwner:  owner,
		argAdd:    add,
		argChange: change,
		argDelete: del,
		argDepend: dep,
	}
}

func (c *UpdateCond) Pre() bool {
	changeIDs := make([]int64, len(c.argChange))
	for i, t := range c.argChange {
		changeIDs[i] = t.ID
	}
	c.preOpen = c.store.IsOpen()
	if c.preOpen {
		c.preChange = c.store.Tasks(changeIDs)
		c.preDelete = c.store.Tasks(c.argDelete)
		c.preDepend = c.store.Tasks(c.argDepend)
	}
	return true
}

func (c *UpdateCond) Call() {
	c.now = NowMillis()
	c.retTasks, c.retErr = c.store.Update(c.argOwner, c.argAdd, c.argChange, c.argDelete, c.argDepend)
}

func (c *UpdateCond) Post() bool {
	if !c.preOpen {
		if c.retErr == nil {
			fmt.Println("Update Postcondition: no error returned when updating a closed store.")
			return false
		}
		return true
	}

	if c.retErr != nil {
		if c.existMissingDependencies() {
			// We expect an error if dependencies are missing.
			return true
		}
		if c.existAlreadyOwned() {
			// We expect an error if changes or deletions are owned elsewhere.
			return true
		}
		fmt.Printf("Update Postcondition: all tasks exist, none are owned by others, but still got an error: %v\n", c.retErr)
		return false
	}

	if c.existMissingDependencies() {
		fmt.Printf("Update Postcondition: no error returned, but missing dependencies: %v\n", c)
		return false
	}
	if c.existAlreadyOwned() {
		fmt.Printf("Update Postcondition: no error returned, but modifications owned by others: %v\n", c)
		return false
	}

	if len(c.retTasks) != len(c.argAdd)+len(c.argChange) {
		fmt.Printf("Update Postcondition: no error returned, but returned tasks not equal to sum of additions and changes: %v != %v + %v\n",
			len(c.retTasks), len(c.argAdd), len(c.argChange))
		return false
	}

	newAdds := c.retTasks[:len(c.argAdd)]
	for i, toAdd := range c.argAdd {
		added := newAdds[i]
		if added.OwnerID != c.argOwner {
			fmt.Printf("Update PostCondition: added task does not have the proper owner set: expected %v, got %v\n", c.argOwner, added.OwnerID)
			return false
		}
		if !c.sameEssentialTask(toAdd, added) {
			fmt.Printf("Update Postcondition: added task differs from requested add: expected\n%v\ngot\n%v\n", toAdd, added)
			return false
		}
		// TODO: In addition to the below, also ensure that the new ID is
		// bigger than any of the existing tasks we were looking at.
		if added.ID == 0 {
			fmt.Printf("Update Postcondition: added task has zero ID: %v\n", added)
			return false
		}
		expectedAT := toAdd.AT
		if toAdd.AT <= 0 {
			expectedAT = c.now - toAdd.AT
		}
		if expectedAT > added.AT+5000 || expectedAT < added.AT-5000 {
			fmt.Printf("Update Postcondition: added task has weird AT: expected\n%v\ngot\n%v\n", toAdd, added)
			return false
		}
	}
	newChanges := c.retTasks[len(c.argAdd):]
	for i, toChange := range c.argChange {
		changed := newChanges[i]
		if changed.OwnerID != c.argOwner {
			fmt.Printf("Update Postcondition: changed task does not have the proper owner set: expected %v, got %v\n", c.argOwner, changed.OwnerID)
			return false
		}
		if !c.sameEssentialTask(toChange, changed) {
			fmt.Printf("Update Postcondition: changed task differs from requested change: expected\n%v\ngot\n%v\n", toChange, changed)
			return false
		}
		if changed.ID <= toChange.ID {
			fmt.Printf("Update Postcondition: changed task should have strictly greater ID:\nrequest\n%v\nresponse\n%v\n", toChange, changed)
			return false
		}
		// Check that the old tasks are all gone.
		oldIDs := make([]int64, len(c.argChange))
		for i, t := range c.argChange {
			oldIDs[i] = t.ID
		}
		oldTasks := c.store.Tasks(oldIDs)
		for _, t := range oldTasks {
			if t != nil {
				fmt.Printf("Update Postcondition: changed a task, but the old task is still present in the task store: %v\n", t)
				return false
			}
		}
	}

	deleted := c.store.Tasks(c.argDelete)
	for i, t := range deleted {
		if t != nil {
			fmt.Printf("Update Postcondition: expected deleted tasks to disapper, but found %d still there.\n",
				c.argDelete[i])
			return false
		}
	}

	groups := c.store.Groups()
	groupMap := make(map[string]struct{})
	for _, g := range groups {
		groupMap[g] = struct{}{}
	}
	for i, t := range c.argAdd {
		if _, ok := groupMap[t.Group]; !ok {
			fmt.Printf("Update Postcondition: added group %s to store, but that group is not present\n", t.Group)
			return false
		}
	}

	// TODO: check that now-empty groups are gone.
	return true
}

// sameEssentialTask compares group and data to see if they are the same. It ignores AT, ID, and OwnerID.
func (c *UpdateCond) sameEssentialTask(t1, t2 *Task) bool {
	for i, d1 := range t1.Data {
		if d1 != t2.Data[i] {
			return false
		}
	}
	return t1.Group == t2.Group
}

func (c *UpdateCond) existAlreadyOwned() bool {
	for _, t := range c.preChange {
		if t.OwnerID != c.argOwner {
			return true
		}
	}
	for _, t := range c.preDelete {
		if t.OwnerID != c.argOwner {
			return true
		}
	}
	return false
}

func (c *UpdateCond) existMissingDependencies() bool {
	for _, t := range c.preChange {
		if t == nil {
			return true
		}
	}
	for _, t := range c.preDelete {
		if t == nil {
			return true
		}
	}
	for _, t := range c.preDepend {
		if t == nil {
			return true
		}
	}
	return false
}

// OpenOpportunisticCond embodies the pre and post conditions for the OpenOpportunistic call.
type OpenOpportunisticCond struct {
	preBusy    bool
	argJournal journal.Interface
	retStore   *TaskStore
	retErr     error
}

func NewOpenOpportunisticCond(journal journal.Interface) *OpenOpportunisticCond {
	return &OpenOpportunisticCond{
		argJournal: journal,
	}
}

func (c *OpenOpportunisticCond) Pre() bool {
	if reflect.ValueOf(c.argJournal).IsNil() {
		fmt.Printf("OpenOpportunistic Precondition: nil journal: %v\n", c.argJournal)
		return false
	}
	return true
}

func (c *OpenOpportunisticCond) Call() {
	c.retStore, c.retErr = OpenOpportunistic(c.argJournal)
}

func (c *OpenOpportunisticCond) Post() bool {
	if c.retErr != nil {
		if c.retStore != nil {
			fmt.Printf("OpenOpportunistic Precondition: error returned, but store exists: %v\n", c.retErr)
			return false
		}
		return true
	}

	if !c.retStore.IsOpen() {
		fmt.Printf("OpenOpportunistic Precondition: successful open, but store is not open: %v\n", c.retStore)
		return false
	}
	if c.retStore.IsStrict() {
		fmt.Printf("OpenOpportunistic Precondition: successful opportunistic open, but in strict mode: %v\n", c.retStore)
		return false
	}
	// TODO: it would be nice to check that the journal and the contents of the
	// store actually match. But this is a lot more work, so we'll skip it for
	// now.
	return true
}

// OpenStrictCond embodies the pre and post conditions for the OpenStrict call.
type OpenStrictCond struct {
	argJournal journal.Interface
	retStore   *TaskStore
	retErr     error
}

func NewOpenStrictCond(journal journal.Interface) *OpenStrictCond {
	return &OpenStrictCond{
		argJournal: journal,
	}
}

func (c *OpenStrictCond) Pre() bool {
	if reflect.ValueOf(c.argJournal).IsNil() {
		fmt.Printf("OpenStrict Precondition: nil journal: %v\n", c.argJournal)
		return false
	}
	return true
}

func (c *OpenStrictCond) Call() {
	c.retStore, c.retErr = OpenStrict(c.argJournal)
}

func (c *OpenStrictCond) Post() bool {
	if c.retErr != nil {
		if c.retStore != nil {
			fmt.Printf("OpenStrict Precondition: error returned, but store exists: %v\n", c.retErr)
			return false
		}
		return true
	}

	if !c.retStore.IsOpen() {
		fmt.Printf("OpenStrict Precondition: successful open, but store is not open: %v\n", c.retStore)
		return false
	}
	if !c.retStore.IsStrict() {
		fmt.Printf("OpenStrict Precondition: successful strict open, but not in strict mode: %v\n", c.retStore)
		return false
	}
	// TODO: it would be nice to check that the journal and the contents of the
	// store actually match. But this is a lot more work, so we'll skip it for
	// now.
	return true
}
