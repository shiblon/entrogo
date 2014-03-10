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
	// A value of 0 indicates that it should be assigned "now" when pushed.
	AvailableTime int64

	// Data holds the data for this task.
	// If you want raw bytes, you'll need to encode them
	// somehow.
	Data string
}

// NewTask creates a new task for this owner and group.
func NewTask(group, data string) *Task {
	return &Task{
		Group: group,
		Data:  data,
	}
}

// Copy performs a shallow copy of this task.
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

// Key returns the ID, to satisfy the keyheap.Item interface. This allows tasks to
// be found and removed from the middle of the heap.
func (t *Task) Key() int64 {
	return t.ID
}
