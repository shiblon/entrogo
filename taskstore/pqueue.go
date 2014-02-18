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

/*
Package taskstore implements a library for a simple task store.

This provides abstractions for creating a simple task store process that
manages data in memory and on disk. It can be used to implement a full-fledged
task queue, but it is only the core storage piece. In particular, it does not
implement any networking protocols.
*/

package taskstore

import (
	"container/heap"
	"fmt"
	"math"
	"math/rand"
	"strings"
)

type TaskQueue struct {
	Name     string
	taskHeap taskQueueImpl
	taskMap  map[int64]*taskItem
	randChan chan float64
}

func NewTaskQueue(name string) *TaskQueue {
	return NewTaskQueueFromTasks(name, []*Task{})
}

func NewTaskQueueFromTasks(name string, tasks []*Task) *TaskQueue {
	q := &TaskQueue{
		Name:     name,
		taskHeap: make([]*taskItem, len(tasks)),
		taskMap:  make(map[int64]*taskItem),
		randChan: make(chan float64, 1),
	}
	for i, t := range tasks {
		ti := &taskItem{index: i, task: t}
		q.taskHeap[i] = ti
		q.taskMap[t.ID] = ti
	}
	// Provide thread-safe random values.
	go func() {
		for {
			q.randChan <- rand.Float64()
		}
	}()

	if q.Len() > 1 {
		heap.Init(&q.taskHeap)
	}
	return q
}

func (t *TaskQueue) String() string {
	hpieces := []string{"["}
	for _, v := range t.taskHeap {
		hpieces = append(hpieces, fmt.Sprintf("   %s", v))
	}
	hpieces = append(hpieces, "]")

	keys := []int64{}
	for k := range t.taskMap {
		keys = append(keys, k)
	}
	mpieces := []string{"["}
	for _, k := range keys {
		ti := t.taskMap[k]
		mpieces = append(mpieces, fmt.Sprintf("   ID %d = index %d", ti.task.ID, ti.index))
	}
	mpieces = append(mpieces, "]")

	return fmt.Sprintf(
		"TQ name=%s\n   heap=%v\n   map=%v\n   chancap=%d",
		t.Name,
		strings.Join(hpieces, "\n   "),
		strings.Join(mpieces, "\n   "),
		cap(t.randChan))
}

func (q *TaskQueue) Push(t *Task) {
	ti := &taskItem{task: t}
	heap.Push(&q.taskHeap, ti)
	q.taskMap[t.ID] = ti
}

func (q *TaskQueue) Pop() *Task {
	ti := heap.Pop(&q.taskHeap).(*taskItem)
	delete(q.taskMap, ti.task.ID)
	return ti.task
}

// PopAt removes an element from the specified index in O(log(n)) time.
func (q *TaskQueue) PopAt(idx int) *Task {
	task := q.PeekAt(idx)
	if task == nil {
		return nil
	}
	// This uses basic heap operations to accomplish removal from the middle. A
	// couple of key things make this possible.
	// - A subslice still points to the underlying array, and has capacity extending to the end.
	// - Adding a smallest element to a prefix heap does not invalidate the rest of the heap.
	// - Pushing an element onto a heap puts it at the end and bubbles it up.
	// So, we take the heap prefix up to but not including idx and push the nil task.
	// This overwrites the element we want to remove with nil (adds to the end
	// of the prefix heap, overwriting underlying array storage, since capacity
	// is still there), and bubbles it to the very top (see Less below, it knows about nil).
	subheap := q.taskHeap[:idx]
	heap.Push(&subheap, &taskItem{task: nil})
	if q.taskHeap[0].task != nil {
		panic("Bubbled nil task to top, but it didn't make it.")
	}
	// Then we remove the nil item at the top.
	heap.Pop(&q.taskHeap)

	delete(q.taskMap, task.ID)
	return task
}

func (q *TaskQueue) Len() int {
	return len(q.taskHeap)
}

func (q *TaskQueue) Peek() *Task {
	return q.PeekAt(0)
}

func (q *TaskQueue) PeekAt(idx int) *Task {
	if idx >= q.Len() {
		return nil
	}
	return q.taskHeap[idx].task
}

func (q *TaskQueue) PeekById(id int64) *Task {
	if ti, ok := q.taskMap[id]; ok {
		return ti.task
	}
	return nil
}

func (q *TaskQueue) PopById(id int64) *Task {
	if ti, ok := q.taskMap[id]; ok {
		return q.PopAt(ti.index)
	}
	return nil
}

// PopRandomAvailable walks the queue randomly choosing a child until it either
// picks one or runs out (and picks the last one before the deadline). If
// deadline <= 0, then there is no deadline.
// Note that this greatly favors "old" tasks, because the probability of
// traversing the tree very far quickly gets vanishingly small.
// There are undoubtedly other interesting approaches to doing this, but it
// seems reasonable for a task store.
func (q *TaskQueue) PopRandomAvailable(deadline int64) *Task {
	// Start at the leftmost location (the lowest value), and randomly jump to
	// children so long as they are earlier than the deadline.
	idx := 0
	chosen := -1
	for idx < q.Len() && q.PeekAt(idx).AT <= deadline {
		left := idx*2 + 1
		right := left + 1
		choices := make([]int, 1, 3)
		choices[0] = idx
		if left < q.Len() && q.PeekAt(left).AT <= deadline {
			choices = append(choices, left)
		}
		if right < q.Len() && q.PeekAt(right).AT <= deadline {
			choices = append(choices, right)
		}
		if len(choices) == 0 {
			break
		}
		choiceIndex := int(math.Floor(<-q.randChan * float64(len(choices))))
		if choiceIndex == 0 {
			chosen = choices[choiceIndex]
			break
		}
		// If we didn't choose the current node, redo the random draw with one of
		// the children as the new heap root.
		idx = choices[choiceIndex]
	}
	if chosen < 0 {
		return nil
	}
	return q.PopAt(chosen)
}

type taskItem struct {
	index int
	task  *Task
}

func (ti *taskItem) String() string {
	return fmt.Sprintf("{%d:%v}", ti.index, ti.task)
}

type taskQueueImpl []*taskItem

func (tq taskQueueImpl) Len() int {
	return len(tq)
}

func (tq taskQueueImpl) Less(i, j int) bool {
	// nil tasks are special: when removing from the middle of the heap, we
	// create a nil task and then use basic heap operations to adjust its
	// location.
	if tq[i].task == nil {
		return true
	} else if tq[j].task == nil {
		return false
	}
	return tq[i].task.AT < tq[j].task.AT
}

func (tq taskQueueImpl) Swap(i, j int) {
	tq[i], tq[j] = tq[j], tq[i]
	tq[i].index = i
	tq[j].index = j
}

func (tq *taskQueueImpl) Push(x interface{}) {
	item := x.(*taskItem)
	item.index = len(*tq)
	*tq = append(*tq, x.(*taskItem))
}

func (tq *taskQueueImpl) Pop() interface{} {
	n := len(*tq)
	item := (*tq)[n-1]
	item.index = -1
	*tq = (*tq)[:n-1]
	return item
}
