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

/*
Package taskstore implements a library for a simple task store.

This provides abstractions for creating a simple task store process that
manages data in memory and on disk. It can be used to implement a full-fledged
task queue, but it is only the core storage piece. It does not, in particular,
implement any networking.
*/
package taskstore

import (
	"fmt"
	"log"
	"strings"
	"time"

	"code.google.com/p/entrogo/taskstore/heap"
)

const (
	// The maximum number of items to deplete from the cache when snapshotting
	// is finished but the cache has items in it (during an update).
	maxCacheDepletion = 20
)

// Task is the atomic task unit. It contains a unique task, an owner ID, and an
// availability time (ms). The data is user-defined and can be basically anything.
//
// 0 (or less) is an invalid ID, and is used to indicate "please assign".
// A negative AvailableTime means "delete this task".
type Task struct {
	ID      int64
	OwnerID int32
	Group   string

	// The milliseconds from the Epoch (UTC) when this task becomes available.
	AvailableTime int64

	// Data holds the data for this task. Must be gob-serializable.
	Data interface{}
}

// NewTask creates a new task for this owner and group.
func NewTask(owner int32, group string) *Task {
	return &Task{
		OwnerID: owner,
		Group:   group,
	}
}

// NewTaskAvailability creates a new task with a specific "AvailableTime",
// meaning that it will become available to be owned by someone else at the
// given number of milliseconds from the epoch.
func NewTaskAvailability(owner int32, group string, at int64) *Task {
	return &Task{
		OwnerID:       owner,
		Group:         group,
		AvailableTime: at,
	}
}

// Copy this task (shallow - if Data is a pointer, it won't get copied).
func (t *Task) Copy() *Task {
	newTask := *t
	return &newTask
}

// String formats this task into a nice string value.
func (t *Task) String() string {
	return fmt.Sprintf("Task %d: g=%q o=%d t=%d d=%#v", t.ID, t.Group, t.OwnerID, t.AvailableTime, t.Data)
}

// Priority returns an integer that can be used for heap ordering.
// In this case it's just the AvailableTime.
func (t *Task) Priority() int64 {
	return t.AvailableTime
}

// Key returns the ID, to satisfy the heap.Item interface. This allows tasks to
// be found and removed from the middle of the heap.
func (t *Task) Key() int64 {
	return t.ID
}

// TaskStore maintains the tasks.
type TaskStore struct {
	// A heap for each group.
	heaps map[string]*heap.Heap

	// All tasks known to this TaskStore.
	tasks map[int64]*Task

	lastTaskID int64

	// When the tasks are being snapshotted, these are used to keep throughput
	// going while the tasks map is put into read-only mode.
	snapshotting bool
	tmpTasks     map[int64]*Task
	delTasks     map[int64]bool

	// To write to the journal opportunistically, push transactions into this
	// channel.
	journalChan chan []updateDiff

	// The journal utility that actually does the work of appending and
	// rotating.
	journaler Journaler
}

// NewOpportunistic returns a new TaskStore instance.
// This store will be opportunistically journaled, meaning that it is possible
// to update, delete, or create a task, get confirmation of it occurring,
// crash, and find that recently committed tasks are lost.
// If task execution is idempotent, this is safe, and is much faster, as it
// writes to disk when it gets a chance.
func NewOpportunistic(journaler Journaler) *TaskStore {
	ts := NewStrict(journaler)
	ts.journalChan = make(chan []updateDiff, 1)
	go func() {
		for {
			ts.journaler.AppendRecord(<-ts.journalChan)
		}
	}()

	return ts
}

// NewStrict returns a TaskStore with journaling done synchronously
// instead of opportunistically. This means that, in the event of a crash, the
// full task state will be recoverable and nothing will be lost that appeared
// to be commmitted.
// Use this if you don't mind slower mutations and really need committed tasks
// to stay committed under all circumstances. In particular, if task execution
// is not idempotent, this is the right one to use.
func NewStrict(journaler Journaler) *TaskStore {
	return &TaskStore{
		heaps:     make(map[string]*heap.Heap),
		tasks:     make(map[int64]*Task),
		tmpTasks:  make(map[int64]*Task),
		delTasks:  make(map[int64]bool),
		journaler: journaler,
	}
}

func (t TaskStore) Journaler() Journaler {
	return t.journaler
}

// String formats this as a string. Shows minimal information like group names.
func (t TaskStore) String() string {
	strs := []string{"TaskStore:", "  groups:"}
	for name := range t.heaps {
		strs = append(strs, fmt.Sprintf("    %q", name))
	}
	strs = append(strs,
		fmt.Sprintf("  snapshotting: %v", t.snapshotting),
		fmt.Sprintf("  num tasks: %d", len(t.tasks)+len(t.tmpTasks)-len(t.delTasks)),
		fmt.Sprintf("  last task id: %d", t.lastTaskID))
	return strings.Join(strs, "\n")
}

// Groups returns a list of all of the groups known to this task store.
func (t TaskStore) Groups() []string {
	g := make([]string, 0, len(t.heaps))
	for n := range t.heaps {
		g = append(g, n)
	}
	return g
}

// nowMillis returns the current time in milliseconds since the UTC epoch.
func nowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond/time.Nanosecond)
}

// getTask returns the task with the given ID if it exists, else nil.
func (t *TaskStore) getTask(id int64) *Task {
	if id <= 0 {
		// Invalid ID.
		return nil
	}
	if _, ok := t.delTasks[id]; ok {
		// Already deleted in the temporary cache.
		return nil
	}
	if t, ok := t.tmpTasks[id]; ok {
		// Sitting in cache.
		return t
	}
	if t, ok := t.tasks[id]; ok {
		// Sitting in the main index.
		return t
	}
	return nil
}

// UpdateError contains a map of errors, the key is the index of a task that
// was not present in an expected way.
type UpdateError struct {
	Errors []error
}

// Error returns an error string (and satisfies the Error interface).
func (ue UpdateError) Error() string {
	strs := []string{"update error:"}
	for _, e := range ue.Errors {
		strs = append(strs, fmt.Sprintf("  %v", e.Error()))
	}
	return strings.Join(strs, "\n")
}

type updateDiff struct {
	OldID   int64
	NewTask *Task
}

func (t *TaskStore) nextID() int64 {
	t.lastTaskID++
	return t.lastTaskID
}

// UpdateDependent updates one task.
// Setting AvailableTime < 0 indicates deletion should occur.
// ID == 0 indicates that this is a new task and it will be assigned a new ID.
// Similarly, an AvailableTime of 0 indicates that the time should be set to now.
// New IDs are returned on success, otherwise an instance of UpdateError is
// returned as the error.
func (t *TaskStore) Update(task *Task) (*Task, error) {
	return t.UpdateDependent(task, nil)
}

// UpdateDependent updates one task with dependencies. Setting AvailableTime <
// 0 indicates deletion should occur.
// ID == 0 indicates that this is a new task and it will be assigned a new ID.
// Similarly, an AvailableTime of 0 indicates that the time should be set to now.
// The dependencies are task IDs that have to exist in order for this to
// succeed. They are merely checked before the operation is completed.
// New IDs are returned on success, otherwise an instance of UpdateError is
// returned as the error.
func (t *TaskStore) UpdateDependent(task *Task, dependencies []int64) (*Task, error) {
	tasks, err := t.UpdateMultipleDependent([]*Task{task}, dependencies)
	return tasks[0], err
}

// UpdateMultiple updates multiple tasks simultaneously. Setting AvailableTime
// < 0 indicates deletion should occur.
// ID == 0 indicates that this is a new task and it will be assigned a new ID.
// Similarly, an AvailableTime of 0 indicates that the time should be set to now.
// New IDs are returned on success, otherwise an instance of UpdateError is
// returned as the error.
func (t *TaskStore) UpdateMultiple(tasks []*Task) ([]*Task, error) {
	return t.UpdateMultipleDependent(tasks, nil)
}

// UpdateMultiple updates multiple tasks simultaneously. Setting AvailableTime
// < 0 indicates deletion should occur.
// ID == 0 indicates that this is a new task and it will be assigned a new ID.
// Similarly, an AvailableTime of 0 indicates that the time should be set to now.
// The dependencies are task IDs that have to exist in order for this to
// succeed. They are merely checked before the operation is completed.
// New IDs are returned on success, otherwise an instance of UpdateError is
// returned as the error.
func (t *TaskStore) UpdateMultipleDependent(updates []*Task, dependencies []int64) ([]*Task, error) {
	// Prepare an error just in case.
	uerr := UpdateError{}

	// Check that the dependencies are all around.
	for _, taskid := range dependencies {
		if task := t.getTask(taskid); task == nil {
			uerr.Errors = append(uerr.Errors, fmt.Errorf("Unmet dependency: task ID %d not present", taskid))
		}
	}

	now := nowMillis()

	transaction := make([]updateDiff, len(updates))

	// Check that the requested operation is allowed.
	// This means:
	// - Adds always work
	// - Updates require that the task ID exists, and that the task is either
	// 		unowned or owned by the requester.
	// - Deletions require that the task ID exists.
	for i, task := range updates {
		if task.ID <= 0 && len(uerr.Errors) == 0 {
			transaction[i] = updateDiff{0, task.Copy()}
			continue // just adding a new task - always allowed
		}
		ot := t.getTask(task.ID)
		if ot == nil {
			uerr.Errors = append(uerr.Errors, fmt.Errorf("Task %d not found", task.ID))
			continue
		}
		if ot.AvailableTime > now && ot.OwnerID != task.OwnerID {
			uerr.Errors = append(uerr.Errors, fmt.Errorf("Task %d owned by client %d, but update requested by client %d", ot.ID, ot.OwnerID, task.OwnerID))
			continue
		}
		// Available time < 0 means delete this task.
		if task.AvailableTime < 0 {
			transaction[i] = updateDiff{task.ID, nil}
		} else {
			transaction[i] = updateDiff{task.ID, task.Copy()}
		}
	}

	if len(uerr.Errors) > 0 {
		return nil, uerr
	}

	// Create new tasks for all non-deleted tasks, since we only get here without errors.
	newTasks := make([]*Task, len(transaction))
	for i, diff := range transaction {
		nt := diff.NewTask
		newTasks[i] = nt
		if nt == nil {
			continue
		}
		// Assign IDs to all new tasks, and assign "now" to any that have no availability set.
		nt.ID = t.nextID()
		if nt.AvailableTime == 0 {
			nt.AvailableTime = now
		}
	}

	t.applyTransaction(transaction)

	return newTasks, nil
}

// startSnapshot takes care of using the journaler to create a snapshot.
func (t *TaskStore) startSnapshot() {
	t.snapshotting = true
	data := make(chan interface{}, 1)
	done := make(chan error, 1)

	go func() {
		done <- t.journaler.Snapshot(data)
	}()

	go func() {
		defer close(data)
	LOOP:
		for _, task := range t.tasks {
			select {
			case data <- task:
				// Yay, do nothing.
			case err := <-done:
				if err != nil {
					log.Printf("snapshot failed: %v", err)
				}
				break LOOP
			}
		}

		// TODO(chris): This is truly running in a new goroutine, so this shared
		// variable should be protected with a mutex or something similar.
		t.snapshotting = false
	}()

}

func (t *TaskStore) journalAppend(transaction []updateDiff) {
	if t.journalChan != nil {
		// Opportunistic
		t.journalChan <- transaction
	} else {
		// Strict
		t.journaler.AppendRecord(transaction)
	}
}

// applyTransaction applies a series of mutations to the task store.
// Each element of the transaction contains information about the old task and
// the new task. Deletions are represented by a new nil task.
func (t *TaskStore) applyTransaction(transaction []updateDiff) {
	// If the journal is about to rotate, here we set the snapshotting flag and
	// kick off the routine that takes a snapshot.
	if t.journaler.ShardFinished() {
		t.startSnapshot()
	}

	t.journalAppend(transaction)

	// Make sure that all records in the transaction see the same value for snapshotting.
	// TODO(chris): protect this with a mutex.
	readonly := t.snapshotting
	for _, diff := range transaction {
		t.applySingleDiff(diff, readonly)
	}

	// Finally, if we are not snapshotting, we can try to move some things out
	// of the cache into the main data section.
	t.synchronousDepleteSomeCache()
}

func (t *TaskStore) synchronousDepleteSomeCache() {
	// We don't go wild with this because it might be pretty big, depending on
	// the length of time the snapshot took, so we just try to ensure progress.
	// TODO(chris): this should happen even when there is no activity on the
	// taskstore. Can we set this up to work without blocking legitimate
	// requests?
	todo := maxCacheDepletion
	if len(t.tmpTasks) > 0 {
		if todo > len(t.tmpTasks) {
			todo = len(t.tmpTasks)
		}
		for id, task := range t.tmpTasks {
			todo--
			if todo < 0 {
				break
			}
			t.tasks[id] = task
			delete(t.tmpTasks, id)
		}
	} else if len(t.delTasks) > 0 {
		if todo > len(t.delTasks) {
			todo = len(t.delTasks)
		}
		for id := range t.delTasks {
			todo--
			if todo < 0 {
				break
			}
			delete(t.tasks, id)
			delete(t.delTasks, id)
		}
	}
}

// applySingleDiff applies one of the updateDiff items in a transaction. If
// readonly is specified, it only writes to the temporary structures and skips
// the main tasks index so that it can remain constant while, e.g., written to
// a snapshot on disk.
func (t *TaskStore) applySingleDiff(diff updateDiff, readonly bool) {
	// If readonly, then we mutate only the temporary maps.
	// Regardless of that status, we always update the heaps.
	ot := t.getTask(diff.OldID)
	nt := diff.NewTask

	if ot != nil {
		delete(t.tmpTasks, ot.ID)
		t.heaps[ot.Group].PopByKey(ot.ID)
		if readonly {
			t.delTasks[ot.ID] = true
		} else {
			delete(t.tasks, ot.ID)
		}
	}
	if nt != nil {
		if readonly {
			t.tmpTasks[nt.ID] = nt
		} else {
			t.tasks[nt.ID] = nt
		}
		t.heapPush(nt)
	}
}

func (t *TaskStore) heapPush(task *Task) {
	h, ok := t.heaps[task.Group]
	if !ok {
		h = heap.New()
		t.heaps[task.Group] = h
	}
	h.Push(task)
}
