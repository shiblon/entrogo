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

// Package taskstore implements a library for a simple task store.
// This provides abstractions for creating a simple task store process that
// manages data in memory and on disk. It can be used to implement a full-fledged
// task queue, but it is only the core storage piece. It does not, in particular,
// implement any networking.
package taskstore

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"code.google.com/p/entrogo/keyheap"
	"code.google.com/p/entrogo/taskstore/journal"
)

const (
	// The maximum number of items to deplete from the cache when snapshotting
	// is finished but the cache has items in it (during an update).
	maxCacheDepletion = 20

	// The maximum number of transactions to journal before snapshotting.
	maxTxnsSinceSnapshot = 30000
)

// TaskStore maintains the tasks.
type TaskStore struct {
	// A heap for each group.
	heaps map[string]*keyheap.KeyHeap

	// All tasks known to this TaskStore.
	tasks map[int64]*Task

	lastTaskID int64

	// When the tasks are being snapshotted, these are used to keep throughput
	// going while the tasks map is put into read-only mode.
	tmpTasks map[int64]*Task
	delTasks map[int64]bool

	// The journal utility that actually does the work of appending and
	// rotating.
	journaler journal.Interface

	// To write to the journal opportunistically, push transactions into this
	// channel.
	journalChan chan []updateDiff

	snapshotting      bool
	txnsSinceSnapshot int

	// Channels for making various requests to the task store.
	updateChan       chan request
	listGroupChan    chan request
	claimChan        chan request
	groupsChan       chan request
	snapshotChan     chan request
	snapshotDoneChan chan error
	stringChan       chan request
}

// NewStrict returns a TaskStore with journaling done synchronously
// instead of opportunistically. This means that, in the event of a crash, the
// full task state will be recoverable and nothing will be lost that appeared
// to be commmitted.
// Use this if you don't mind slower mutations and really need committed tasks
// to stay committed under all circumstances. In particular, if task execution
// is not idempotent, this is the right one to use.
func NewStrict(journaler journal.Interface) *TaskStore {
	return newTaskStoreHelper(journaler, false)
}

// NewOpportunistic returns a new TaskStore instance.
// This store will be opportunistically journaled, meaning that it is possible
// to update, delete, or create a task, get confirmation of it occurring,
// crash, and find that recently committed tasks are lost.
// If task execution is idempotent, this is safe, and is much faster, as it
// writes to disk when it gets a chance.
func NewOpportunistic(journaler journal.Interface) *TaskStore {
	return newTaskStoreHelper(journaler, true)
}

func newTaskStoreHelper(journaler journal.Interface, opportunistic bool) *TaskStore {
	ts := &TaskStore{
		journaler: journaler,

		heaps:    make(map[string]*keyheap.KeyHeap),
		tasks:    make(map[int64]*Task),
		tmpTasks: make(map[int64]*Task),
		delTasks: make(map[int64]bool),

		updateChan:       make(chan request),
		listGroupChan:    make(chan request),
		claimChan:        make(chan request),
		groupsChan:       make(chan request),
		snapshotChan:     make(chan request),
		snapshotDoneChan: make(chan error),
		stringChan:       make(chan request),
	}

	var err error

	// Before starting our handler and allowing requests to come in, try
	// reading the latest snapshot and journals.
	sdec, err := journaler.SnapshotDecoder()
	if err != nil && err != io.EOF {
		panic(fmt.Sprintf("snapshot error: %v", err))
	}
	jdec, err := journaler.JournalDecoder()
	if err != nil && err != io.EOF {
		panic(fmt.Sprintf("journal error: %v", err))
	}

	// Read the snapshot.
	task := new(Task)
	err = sdec.Decode(task)
	for err != io.EOF {
		if _, ok := ts.tasks[task.ID]; ok {
			panic(fmt.Sprintf("can't happen - two tasks with same ID %d in snapshot", task.ID))
		}
		ts.tasks[task.ID] = task
		if ts.lastTaskID < task.ID {
			ts.lastTaskID = task.ID
		}
		if _, ok := ts.heaps[task.Group]; !ok {
			ts.heaps[task.Group] = keyheap.New()
		}
		ts.heaps[task.Group].Push(task)

		// Get ready for a new round.
		task = new(Task)
		err = sdec.Decode(task)
		if err != nil && err != io.EOF {
			panic(fmt.Sprintf("corrupt snapshot: %v", err))
		}
	}

	// Replay the journals.
	transaction := new([]updateDiff)
	err = jdec.Decode(transaction)
	for err != io.EOF && err != io.ErrUnexpectedEOF {
		// Replay this transaction - not busy snapshotting.
		ts.playTransaction(*transaction, false)
		ts.txnsSinceSnapshot++

		// Next record
		transaction = new([]updateDiff)
		err = jdec.Decode(transaction)
		if err == io.ErrUnexpectedEOF {
			log.Println("Found unexpected EOF in journal stream. Continuing.")
		} else if err != nil && err != io.EOF {
			panic(fmt.Sprintf("corrupt journal: %v", err))
		}
	}

	if opportunistic {
		// non-nil journalChan means "append opportunistically" and frees up
		// the journalChan case in "handle".
		// TODO: is this the right buffer size?
		ts.journalChan = make(chan []updateDiff, 1)
	}

	// Everything is ready, now we can start our request handling loop.
	go ts.handle()
	return ts
}

// String formats this as a string. Shows minimal information like group names.
func (t *TaskStore) String() string {
	resp := t.sendRequest(nil, t.stringChan)
	return resp.Val.(string)
}

// NowMillis returns the current time in milliseconds since the UTC epoch.
func NowMillis() int64 {
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

// snapshot takes care of using the journaler to create a snapshot.
func (t *TaskStore) snapshot() error {
	// first we make sure that the cache is flushed. We're still synchronous,
	// because we're in the main handler and no goroutines have been created.

	t.depleteCache(0)
	if len(t.tmpTasks) + len(t.delTasks) > 0 {
		panic("depleted cache in synchronous code, but not depleted. should never happen.")
	}

	data := make(chan interface{}, 1)
	done := make(chan error, 1)
	snapresp := make(chan error, 1)

	go func() {
		done <- t.journaler.StartSnapshot(data, snapresp)
	}()

	go func() {
		var err error
		defer func() {
			// Notify completion of this asynchronous snapshot.
			t.snapshotDoneChan <- err
		}()
		defer close(data)

		for _, task := range t.tasks {
			select {
			case data <- task:
				// Yay, data sent.
			case err = <-done:
				return // errors are sent out in defer
			case err = <-snapresp:
				return // errors are sent out in defer
			}
		}
	}()

	return nil
}

func (t *TaskStore) journalAppend(transaction []updateDiff) {
	if t.journalChan != nil {
		// Opportunistic
		t.journalChan <- transaction
	} else {
		// Strict
		t.doAppend(transaction)
	}
}

func (t *TaskStore) doAppend(transaction []updateDiff) {
	t.journaler.Append(transaction)
	t.txnsSinceSnapshot++
}

// applyTransaction applies a series of mutations to the task store.
// Each element of the transaction contains information about the old task and
// the new task. Deletions are represented by a new nil task.
func (t *TaskStore) applyTransaction(transaction []updateDiff) {
	t.journalAppend(transaction)
	t.playTransaction(transaction, t.snapshotting)
}

// playTransaction applies the diff (a series of mutations that should happen
// together) to the in-memory database. It does not journal.
func (t *TaskStore) playTransaction(tx []updateDiff, ro bool) {
	for _, diff := range tx {
		t.applySingleDiff(diff, ro)
	}
}

// depleteCache tries to move some of the elements in
// temporary structures into the main data area.
// Passing a number <= 0 indicates full depletion.
func (t *TaskStore) depleteCache(todo int) {
	if todo <= 0 {
		todo = len(t.tmpTasks) + len(t.delTasks)
	}
	for ; todo > 0; todo-- {
		switch {
		case len(t.tmpTasks) > 0:
			for id, task := range t.tmpTasks {
				t.tasks[id] = task
				delete(t.tmpTasks, id)
				break // just do one
			}
		case len(t.delTasks) > 0:
			for id := range t.delTasks {
				delete(t.tasks, id)
				delete(t.delTasks, id)
				break // just do one
			}
		default:
			todo = 0 // nothing left above
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
		t.heapPop(ot.Group, ot.ID)
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

// heapPop takes the specified task ID out of the heaps, and removes the group
// if it is now empty.
func (t *TaskStore) heapPop(group string, id int64) {
	h, ok := t.heaps[group]
	if !ok {
		return
	}
	h.PopByKey(id)
	if h.Len() == 0 {
		delete(t.heaps, group)
	}
}

// heapPush pushes something onto the heap for the task's group. Creates a new
// group if this one does not already exist.
func (t *TaskStore) heapPush(task *Task) {
	h, ok := t.heaps[task.Group]
	if !ok {
		h = keyheap.New()
		t.heaps[task.Group] = h
	}
	h.Push(task)
}

// update performs (or attempts to perform) a batch task update.
func (t *TaskStore) update(up reqUpdate) ([]*Task, error) {
	uerr := UpdateError{}
	transaction := make([]updateDiff, len(up.Changes))

	// Check that the dependencies are all around.
	for _, id := range up.Dependencies {
		if task := t.getTask(id); task == nil {
			uerr.Errors = append(uerr.Errors, fmt.Errorf("unmet dependency: task ID %d not present", id))
		}
	}

	now := NowMillis()

	// Check that the requested operation is allowed.
	// This means:
	// - Insertions are always allowed
	// - Updates require that the task ID exists, and that the task is either
	// 		unowned or owned by the requester.
	// - Deletions require that the task ID exists.
	for i, task := range up.Changes {
		if task.ID <= 0 && len(uerr.Errors) == 0 {
			if task.Group == "" {
				uerr.Errors = append(uerr.Errors, fmt.Errorf("adding task with empty task group not allowed"))
				continue
			}
			transaction[i] = updateDiff{0, task.Copy()}
			continue
		}
		ot := t.getTask(task.ID)
		if ot == nil {
			uerr.Errors = append(uerr.Errors, fmt.Errorf("task %d not found", task.ID))
			continue
		}
		if ot.AvailableTime > now && ot.OwnerID != up.OwnerID {
			err := fmt.Errorf("task %d owned by %d, cannot be changed by %d", ot.ID, ot.OwnerID, up.OwnerID)
			uerr.Errors = append(uerr.Errors, err)
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
	// Also assign IDs and times as needed.
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

func (t *TaskStore) listGroup(lg reqListGroup) []*Task {
	h, ok := t.heaps[lg.Name]
	if !ok {
		// A non-existent group is actually not a problem. Groups are deleted
		// when empty and lazily created, so we allow them to simply come up
		// empty.
		return nil
	}
	limit := lg.Limit
	if limit <= 0 || limit > h.Len() {
		limit = h.Len()
	}
	var tasks []*Task
	if lg.AllowOwned {
		tasks = make([]*Task, limit)
		for i := range tasks {
			tasks[i] = h.PeekAt(i).(*Task)
		}
	} else {
		now := NowMillis()
		tasks = make([]*Task, 0, limit)
		for i, found := 0, 0; i < h.Len() && found < limit; i++ {
			task := h.PeekAt(i).(*Task)
			if task.AvailableTime <= now {
				tasks = append(tasks, task)
				found++
			}
		}
	}
	return tasks
}

func (t *TaskStore) groups() []string {
	groups := make([]string, 0, len(t.heaps))
	for k := range t.heaps {
		groups = append(groups, k)
	}
	return groups
}

func (t *TaskStore) claim(claim reqClaim) ([]*Task, error) {
	now := NowMillis()
	nmap := make(map[string]bool)
	for _, name := range claim.Names {
		if _, ok := nmap[name]; ok {
			return nil, fmt.Errorf("duplicate name %q requested claiming tasks from groups %v", name, claim.Names)
		}
		nmap[name] = true
	}

	duration := claim.Duration
	if duration < 0 {
		duration = 0
	}

	// Check that the tasks exist and are ready to be owned.
	for _, name := range claim.Names {
		h := t.heaps[name]
		if h == nil || h.Len() == 0 {
			return nil, fmt.Errorf("no tasks in group %q to claim", name)
		}
		task := h.Peek().(*Task)
		if task.AvailableTime > now {
			return nil, fmt.Errorf("no unowned tasks in group %q to claim", name)
		}
	}

	// We can proceed, so we create task updates randomly from every group.
	tasks := make([]*Task, len(claim.Names))
	for i, name := range claim.Names {
		task := t.heaps[name].PopRandomConstrained(now).(*Task)
		// Create a mutated task that shares data and ID with this one, and
		// we'll request it to have these changes.
		tasks[i] = &Task{
			ID:            task.ID,
			OwnerID:       claim.OwnerID,
			Group:         name,
			AvailableTime: now + duration,
			Data:          task.Data,
		}
	}

	// Because claiming involves setting the owner and a future availability,
	// we update these acquired tasks.
	up := reqUpdate{
		OwnerID:      claim.OwnerID,
		Changes:      tasks,
		Dependencies: nil,
	}
	return t.update(up)
}

// Update makes changes to the task store. The owner is the ID of the
// requester, and tasks to be added, changed, and deleted can be specified. If
// dep is specified, it is a list of task IDs that must be present for the
// update to succeed.
func (t *TaskStore) Update(owner int32, add, change []*Task, del []int64, dep []int64) ([]*Task, error) {
	up := reqUpdate{
		OwnerID:      owner,
		Changes:      make([]*Task, 0, len(add)+len(change)+len(del)),
		Dependencies: dep,
	}

	for _, task := range add {
		task := task.Copy()
		task.ID = 0          // ensure that it's really an add.
		task.OwnerID = owner // require that the owner be the requester.
		if task.AvailableTime < 0 {
			task.AvailableTime = 0 // ensure that it doesn't get marked for deletion.
		}
		up.Changes = append(up.Changes, task)
	}

	for _, task := range change {
		task := task.Copy()
		task.OwnerID = owner
		if task.AvailableTime < 0 {
			task.AvailableTime = 0 // no accidental deletions
		}
		up.Changes = append(up.Changes, task)
	}

	for _, id := range del {
		// Create a deletion task.
		up.Changes = append(up.Changes, &Task{ID: id, OwnerID: owner, AvailableTime: -1})
	}

	resp := t.sendRequest(up, t.updateChan)
	return resp.Val.([]*Task), resp.Err
}

// ListGroup tries to find tasks for the given group name. The number of tasks
// returned will be no more than the specified limit. A limit of 0 or less
// indicates that all possible tasks should be returned. If allowOwned is
// specified, then even tasks with AvailableTime in the future that are owned
// by other clients will be returned.
func (t *TaskStore) ListGroup(name string, limit int, allowOwned bool) []*Task {
	lg := reqListGroup{
		Name:       name,
		Limit:      limit,
		AllowOwned: allowOwned,
	}
	resp := t.sendRequest(lg, t.listGroupChan)
	return resp.Val.([]*Task)
}

// Groups returns a list of all of the groups known to this task store.
func (t *TaskStore) Groups() []string {
	resp := t.sendRequest(nil, t.groupsChan)
	return resp.Val.([]string)
}

// Claim attempts to find one random unowned task in each of the specified
// group names and set the ownership to the specified owner. If successful, the
// newly-owned tasks are returned with their AvailableTime set to now +
// duration (in milliseconds).
func (t *TaskStore) Claim(owner int32, names []string, duration int64) ([]*Task, error) {
	claim := reqClaim{
		OwnerID:  owner,
		Names:    names,
		Duration: duration,
	}
	resp := t.sendRequest(claim, t.claimChan)
	return resp.Val.([]*Task), resp.Err
}

// Snapshot tries to force a snapshot to start immediately. It only fails if
// there is already one in progress.
func (t *TaskStore) Snapshot() error {
	resp := t.sendRequest(nil, t.snapshotChan)
	return resp.Err
}

// reqUpdate contains the necessary fields for requesting an update to a
// set of tasks, including changes, deletions, and tasks on whose existence the
// update depends.
type reqUpdate struct {
	OwnerID      int32
	Changes      []*Task
	Dependencies []int64
}

// reqListGroup is a query for up to Limit tasks in the given group name. If <=
// 0, all tasks are returned.
type reqListGroup struct {
	Name       string
	Limit      int
	AllowOwned bool
}

// reqClaim is a query for claiming one task from each of the specified groups.
type reqClaim struct {
	// The owner that is claiming the task.
	OwnerID int32

	// Names are the group names for which tasks should be claimed. If any of
	// them does not have a claimable task, the entire operation fails.
	Names []string

	// Duration is in milliseconds. The task availability, if claimed, will
	// become now + Duration.
	Duration int64
}

// request wraps a query structure, and is used internally to handle the
// multi-channel request/response protocol.
type request struct {
	Val        interface{}
	ResultChan chan response
}

// response wraps a value by adding an error to it.
type response struct {
	Val interface{}
	Err error
}

// sendRequest sends val on the channel ch and waits for a response.
func (t *TaskStore) sendRequest(val interface{}, ch chan request) response {
	req := request{
		Val:        val,
		ResultChan: make(chan response, 1),
	}
	ch <- req
	return <-req.ResultChan
}

// handle deals with all of the basic operations on the task store. All outside
// requests come through this single loop, which is part of the single-threaded
// access design enforcement.
func (t *TaskStore) handle() {
	idler := time.Tick(5 * time.Second)
	for {
		select {
		case req := <-t.updateChan:
			tasks, err := t.update(req.Val.(reqUpdate))
			if err == nil {
				// Successful txn: is it time to create a full snapshot?
				if t.txnsSinceSnapshot >= maxTxnsSinceSnapshot {
					t.txnsSinceSnapshot = 0
					t.snapshot()
				}

				// Finally, if we are not snapshotting, we can try to move some things out
				// of the cache into the main data section. On the off chance that
				// snapshotting finished by now, we check it again.
				if !t.snapshotting {
					t.depleteCache(maxCacheDepletion)
				}
			}
			req.ResultChan <- response{tasks, err}
		case req := <-t.listGroupChan:
			tasks := t.listGroup(req.Val.(reqListGroup))
			req.ResultChan <- response{tasks, nil}
		case req := <-t.claimChan:
			tasks, err := t.claim(req.Val.(reqClaim))
			req.ResultChan <- response{tasks, err}
		case req := <-t.groupsChan:
			groups := t.groups()
			req.ResultChan <- response{groups, nil}
		case req := <-t.snapshotChan:
			if t.snapshotting {
				err := fmt.Errorf("attempted snapshot while already snapshotting")
				req.ResultChan <- response{nil, err}
				break // out of select
			}
			t.snapshotting = true
			err := t.snapshot()
			req.ResultChan <- response{nil, err}
		case err := <-t.snapshotDoneChan:
			if err != nil {
				// TODO: Should this really be fatal?
				panic(fmt.Sprintf("snapshot failed: %v", err))
			}
			t.snapshotting = false
		case <-idler:
			// The idler got a chance to tick. Trigger a short depletion.
			t.depleteCache(maxCacheDepletion)
		case transaction := <-t.journalChan:
			// Opportunistic journaling.
			// TODO: will journaling fall behind due to starvation in here? Should opportunistic be asynchronous?
			t.doAppend(transaction)
		case req := <-t.stringChan:
			strs := []string{"TaskStore:", "  groups:"}
			for name := range t.heaps {
				strs = append(strs, fmt.Sprintf("    %q", name))
			}
			strs = append(strs,
				fmt.Sprintf("  snapshotting: %v", t.snapshotting),
				fmt.Sprintf("  num tasks: %d", len(t.tasks)+len(t.tmpTasks)-len(t.delTasks)),
				fmt.Sprintf("  last task id: %d", t.lastTaskID))
			req.ResultChan <- response{strings.Join(strs, "\n"), nil}
		}
	}
}
