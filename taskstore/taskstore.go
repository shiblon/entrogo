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
	"time"

	"code.google.com/p/entrogo/taskstore/tqueue"
)

type taskStore struct {
	journalDir string
	groups     map[string]tqueue.TaskQueue
}

func New(journalDir string) *taskStore {
	// TODO: ensure that the directory either exists or can be created.
	return &taskStore{
		journalDir: journalDir,
	}
}

func (t *taskStore) Init() {
	// TODO: check that it is not already initialized.
	//
	// Create a single task in the empty group with the last usable task ID in it (initially 0).
}

func (t *taskStore) Start() {
	// TODO: ensure that there can only be one instance of this thing.
	// How do we do that? Using the file system? Some OS-level thing?
	// TODO
	// Look for the journal directory and replay any files there.
}

// getTask returns the task in the given group with the given ID.
func (t *taskStore) getTask(group string, id int64) (task *tqueue.Task, err error) {
	g, ok := t.groups[group]
	if !ok {
		return nil, fmt.Errorf("No such group %q in taskstore", group)
	}
	task = g.PeekById(id)
	if task == nil {
		return nil, fmt.Errorf("No such task %d in group %q in taskstore", id, group)
	}
	return task, nil
}

func (t *taskStore) nowMicroseconds() int64 {
	return time.Now().UnixNano() / 1000
}

// Update updates the tasks specified in the tasks list. If the AT for any task
// is negative, it indicates that the task should be deleted.
// In the event of success, the new IDs are returned. If the operation was not
// successful, the list of errors will contain at least one non-nil entry.
func (t *taskStore) Update(tasks []tqueue.Task) (newIds []int64, errors []error) {
	newIds = make([]int64, len(tasks))
	errors = make([]error, len(tasks))
	oldTasks := make([]*tqueue.Task, len(tasks))

	hasErrors := false

	now := t.nowMicroseconds()

	for i, task := range tasks {
		if task.ID == 0 {
			continue // just adding a new task
		}
		ot, err := t.getTask(task.Group, task.ID)
		errors[i] = err
		oldTasks[i] = ot
		if errors[i] == nil {
			if ot.AT > now && ot.Owner != task.Owner {
				errors[i] = fmt.Errorf("Task %d in group %q is owned by %d, but update request came from %d", ot.ID, ot.Group, ot.Owner, task.Owner)
			}
		} else {
			hasErrors = true
		}
	}

	if hasErrors {
		return newIds, errors
	}

	// If we got this far, then we're good to go. We can update all of the
	// tasks with new IDs and new content. If AT is zero, we set it to now().
	// If it is negative, we delete the task.
	for _, task := range tasks {
		g := t.groups[task.Group]
		if task.AT == 0 {
			task.AT = now
		}
		// TODO(chris): get a new ID
		g.PopById(task.ID)
		if task.AT >= 0 {
			g.Push(&task)
			// TODO(chris): send an update to the log
		} else {
			// TODO(chris): send a tombstone to the log
		}
	}
	return newIds, nil
}
