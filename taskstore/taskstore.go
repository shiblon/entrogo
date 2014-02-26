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
	"sync"
	"time"

	"code.google.com/p/entrogo/taskstore/heap"
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

// Copy this task (shallow - if Data is a pointer, it won't get copied).
func (t *Task) Copy() *Task {
	newTask := *t
	return &newTask
}

// String formats this task into a nice string value.
func (t *Task) String() string {
	return fmt.Sprintf("Task %d: g=%s o=%d t=%d d=%#v", t.ID, t.Group, t.OwnerID, t.AvailableTime, t.Data)
}

// Priority returns an integer that can be used for heap ordering.
// In this case it's just the AvailableTime.
func (t *Task) Priority() int64 {
	return t.AvailableTime
}

// TaskStore maintains the tasks.
type TaskStore struct {
	sync.RWMutex

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
}

// un of the "defer un(lock(x))" pattern.
// Takes a niladic function that is supposed to undo whatever was done.
// In the case of "lock", it would "unlock". In the case of something like
// "trace", it would "untrace".
func un(f func()) {
	f()
}

// lock of the "defer un(lock(x))" pattern.
// Returns a function to call that will unlock this locker.
func lock(locker sync.Locker) func() {
	locker.Lock()
	return func() { locker.Unlock() }
}

// New returns a new TaskStore instance.
func New() *TaskStore {
	return &TaskStore{
		heaps:      make(map[string]*heap.Heap),
		tasks:      make(map[int64]*Task),
		groupNames: []string{"--internal--"}, // Group 0 is special.
		tmpTasks:   make(map[int64]*Task),
		delTasks:   make(map[int64]bool),
	}
}

// nowMillis returns the current time in milliseconds since the UTC epoch.
func nowMillis() int64 {
	return time.Now().UnixNano() / (time.Millisecond / time.Nanosecond)
}

// getTask returns the task with the given ID if it exists.
func (t *TaskStore) getTask(id int64) (*Task, error) {
	if t, ok := t.delTasks[id]; ok {
		return nil, fmt.Errorf("Task %d not found", id)
	}
	if t, ok := t.tmpTasks[id]; ok {
		return t, nil
	}
	if t, ok := t.tasks[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("Task %d not found", id)
}

// UpdateError contains a map of errors, the key is the index of a task that
// was not present in an expected way.
type UpdateError struct {
	Errors map[int64]error
}

// Error returns an error string (and satisfies the Error interface).
func (ue UpdateError) Error() string {
	// TODO(chris): fill this in
}

type updateDiff struct {
	OldId   int64
	NewTask *Task
}

func (t *TaskStore) nextID() int64 {
	t.lastID++
	return t.lastID
}

// Update updates the tasks specified in the tasks list. If the AvailableTime
// for any task <= 0 it indicates that the task should be deleted. In the
// event of success, the new IDs are returned. If the operation was not
// successful, the list of errors will contain at least one non-nil entry.
// If a task has ID <= 0, it is assumed that an ID needs to be assigned and the
// task is being added.
// The dependencies are task IDs that have to exist in order for this to
// succeed. They are merely checked before the operation is completed.
func (t *TaskStore) Update(updates []*Task, dependencies []int64) ([]*Task, error) {
	// TODO(chris): Figure out what kind of lock to hold, here (if any), so
	// that we are sure to do the right thing with snapshots and journaling and
	// whatnot.
	// Prepare an error just in case.
	uerr := UpdateError{
		Errors: make([]error, len(updates)),
	}

	// Check that the dependencies are all around.
	for i, taskid := range dependencies {
		if _, err := t.getTask(taskid); err != nil {
			uerr.Errors[taskid] = fmt.Errorf("Unmet dependency: task ID %d not present", taskid)
		}
	}

	now := nowMillis()

	transaction := make([]updatedDiff, len(updates))

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
		ot, err := t.getTask(task.Group, task.ID)
		if err != nil {
			uerr.Errors[task.ID] = err
			continue
		}
		if ot.AvailableTime > now && ot.OwnerID != task.OwnerID {
			uerr.Errors[task.ID] = fmt.Errorf("Task %d owned by client %d, but update requested by client %d", ot.ID, ot.OwnerID, task.OwnerID)
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
	// TODO: should we create new tasks instead of altering the arguments?
	// It might be more polite...
	newTasks := make([]*Task, len(transactions))
	for i, diff := range transactions {
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

// applyTransaction applies a series of mutations to the task store.
// Each element of the transaction contains information about the old task and
// the new task. Deletions are represented by a new nil task.
//
// This should only be called while the write lock is held.
func (t *TaskStore) applyTransaction(transaction []updateDiff) {
	// TODO(chris): journal this.
	readonly := t.snapshotting
	for _, diff := range transaction {
		t.applySingleDiff(diff, readonly)
	}
}

func (t *TaskStore) applySingleDiff(diff updateDiff, readonly bool) {
	// If readonly, then we mutate only the temporary maps.
	// Regardless of that status, we always update the heaps and the journal.
	oid := updateDiff.OldID
	nt := updateDiff.NewTask

	if readonly {
		if oid > 0 {
			t.delTasks[oid] = true
			delete(t.tmpTasks, oid)
		}
		if nt != nil {
			t.tmpTasks[nt.ID] = nt
		}
		return
	}

	// Not readonly, mutate cache *and* main data.
	if oid > 0 {
		delete(t.tmpTasks, oid)
		delete(t.tasks, oid)
	}
	if nt != nil {
		t.tasks[nt.ID] = nt
	}
}
