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
	"reflect"

	"code.google.com/p/entrogo/taskstore/journal"
)

// Pre/Post conditions for various API calls.
// These are used to represent what should happen when a call is made to an
// API function, depending on the state of the world before it happens.
type Condition interface {
	// Pre is called before calling the Call function. If it returns false,
	// the Call function is not called because the precondition is not met.
	// You can pass in information to aid in providing state if needed.
	Pre(info interface{}) bool

	// Call calls the API corresponding to this condition (e.g., Claim)
	Call()

	// Post is called after Call, and indicates whether the Call left things
	// in an appropriate state.
	Post() bool
}

// A ClaimCond embodies the pre and post conditions for calling TaskStore.Claim.
type ClaimCond struct {
	Store *TaskStore

	PreDepend []*Task

	ArgOwner    int32
	ArgGroup    string
	ArgDuration int64
	ArgDepend   []int64

	RetTask *Task
	RetErr  error

	PreNow            int64
	PreOpen           bool
	PreUnownedInGroup int
}

func NewClaimCond(owner int32, group string, duration int64, depends []int64) *ClaimCond {
	return &ClaimCond{
		ArgOwner:    owner,
		ArgGroup:    group,
		ArgDuration: duration,
		ArgDepend:   depends,
	}
}

func (c *ClaimCond) Pre(info interface{}) bool {
	c.Store = info.(*TaskStore)
	c.PreNow = NowMillis()
	c.PreOpen = c.Store.IsOpen()
	if c.PreOpen {
		c.PreUnownedInGroup = len(c.Store.ListGroup(c.ArgGroup, 0, false))
		c.PreDepend = c.Store.Tasks(c.ArgDepend)
	}
	return true
}

func (c *ClaimCond) Call() {
	c.RetTask, c.RetErr = c.Store.Claim(c.ArgOwner, c.ArgGroup, c.ArgDuration, c.ArgDepend)
}

func (c *ClaimCond) Post() bool {
	if !c.PreOpen {
		if c.Store.IsOpen() {
			fmt.Println("Claim Postcondition: store was not open, but is now")
			return false
		}
		if c.RetErr == nil {
			fmt.Println("Claim Postcondition: no error returned when claiming from a closed store")
			return false
		}
		if numUnowned := len(c.Store.ListGroup(c.ArgGroup, 0, false)); numUnowned != c.PreUnownedInGroup {
			fmt.Println("Claim Postcondition: unowned tasks changed, even though the store is closed")
			return false
		}
		return true
	}

	// Check that failed dependencies cause a claim error
	var hasNil int64 = -1
	for i, task := range c.PreDepend {
		if task == nil {
			hasNil = c.ArgDepend[i]
			break
		}
	}
	if hasNil >= 0 && c.RetErr == nil {
		fmt.Printf("Claim Postcondition: dependency %d missing, but claim succeeded.\n", hasNil)
		return false
	}

	now := NowMillis()
	numUnowned := len(c.Store.ListGroup(c.ArgGroup, 0, false))
	if c.PreUnownedInGroup == 0 {
		if numUnowned > 0 {
			fmt.Println("Claim Postcondition: no tasks to claim, magically produced a claimable task")
			return false
		}
		if c.RetErr != nil {
			fmt.Printf("Claim Postcondition: should not be an error to claim no tasks when non exist, but got %v.\n", c.RetErr)
			return false
		}
		return true
	}

	if c.RetTask == nil || c.RetErr != nil {
		fmt.Printf("Claim Postcondition: unowned tasks available, but none claimed: error %v\n", c.RetErr)
		return false
	}

	if c.RetTask.AT > now && numUnowned != (c.PreUnownedInGroup-1) {
		fmt.Println("Claim Postcondition: unowned expected to change by -1, changed by %d",
			c.PreUnownedInGroup-numUnowned)
		return false
	}

	if c.RetTask.OwnerID != c.ArgOwner {
		fmt.Println("Claim Postcondition: owner not assigned properly to claimed task")
		return false
	}

	// TODO: check that the claimed task is actually one of the unowned tasks.

	return true
}

// A CloseCond embodies the pre and post conditions for the Close call.
type CloseCond struct {
	Store   *TaskStore
	PreOpen bool
	RetErr  error
}

func NewCloseCond() *CloseCond {
	return &CloseCond{
	}
}

func (c *CloseCond) Pre(info interface{}) bool {
	c.Store = info.(*TaskStore)
	c.PreOpen = c.Store.IsOpen()
	return true
}

func (c *CloseCond) Call() {
	c.RetErr = c.Store.Close()
}

func (c *CloseCond) Post() bool {
	postOpen := c.Store.IsOpen()
	if !c.PreOpen {
		if postOpen {
			fmt.Println("Close Postcondition: magically opened from closed state.")
			return false
		}
		if c.RetErr == nil {
			fmt.Println("Close Postcondition: closed store, but Close did not return an error.")
			return false
		}
		return true
	}
	if c.RetErr != nil {
		fmt.Printf("Close Postcondition: closed an open store, but got an error: %v\n", c.RetErr)
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
	Store         *TaskStore
	PreOpen       bool
	PreNow        int64
	ArgGroup      string
	ArgLimit      int
	ArgAllowOwned bool
	RetTasks      []*Task
}

func NewListGroupCond(group string, limit int, allowOwned bool) *ListGroupCond {
	return &ListGroupCond{
		ArgGroup:      group,
		ArgLimit:      limit,
		ArgAllowOwned: allowOwned,
	}
}

func (c *ListGroupCond) Pre(info interface{}) bool {
	c.Store = info.(*TaskStore)
	c.PreOpen = c.Store.IsOpen()
	c.PreNow = NowMillis()
	return true
}

func (c *ListGroupCond) Call() {
	c.RetTasks = c.Store.ListGroup(c.ArgGroup, c.ArgLimit, c.ArgAllowOwned)
}

func (c *ListGroupCond) Post() bool {
	if !c.PreOpen {
		if c.RetTasks != nil {
			fmt.Println("ListGroup Postcondition: returned non-nil tasks.")
			return false
		}
	}
	if c.ArgLimit <= 0 {
		return true // we can't really test this separately from itself.
	}
	if !c.ArgAllowOwned {
		for _, t := range c.RetTasks {
			// Not allowing owned, but got owned tasks anyway.
			if t.AT > c.PreNow {
				fmt.Println("ListGroup Postcondition: got owned tasks when not asking for them.")
				return false
			}
		}
	}
	if len(c.RetTasks) > c.ArgLimit {
		fmt.Printf("ListGroup Postcondition: asked for max %d tasks, got more (%d).\n", c.ArgLimit, len(c.RetTasks))
		return false
	}
	return true
}

// GroupsCond embodies the pre and post conditions for the store's Groups call.
type GroupsCond struct {
	Store     *TaskStore
	PreOpen   bool
	RetGroups []string
}

func NewGroupsCond() *GroupsCond {
	return &GroupsCond{
	}
}

func (c *GroupsCond) Pre(info interface{}) bool {
	c.Store = info.(*TaskStore)
	c.PreOpen = c.Store.IsOpen()
	return true
}

func (c *GroupsCond) Call() {
	c.RetGroups = c.Store.Groups()
}

func (c *GroupsCond) Post() bool {
	if !c.PreOpen {
		if c.RetGroups != nil {
			fmt.Println("Groups Postcondition: returned non-nil groups when closed.")
			return false
		}
		return true
	}
	if c.RetGroups == nil {
		fmt.Println("Groups Postcondition: returned nil groups when open.")
		return false
	}
	return true
}

// NumTasksCond embodies the pre and post conditions for the NumTasks call.
type NumTasksCond struct {
	Store   *TaskStore
	PreOpen bool
	RetNum  int32
}

func NewNumTasksCond() *NumTasksCond {
	return &NumTasksCond{
	}
}

func (c *NumTasksCond) Pre(info interface{}) bool {
	c.Store = info.(*TaskStore)
	c.PreOpen = c.Store.IsOpen()
	return true
}

func (c *NumTasksCond) Call() {
	c.RetNum = c.Store.NumTasks()
}

func (c *NumTasksCond) Post() bool {
	if !c.PreOpen {
		if c.RetNum != 0 {
			fmt.Println("NumTasks Postcondition: non-zero tasks on closed store.")
			return false
		}
		return true
	}
	if c.RetNum < 0 {
		fmt.Println("NumTasks Postcondition: negative task num returned.")
		return false
	}
	return true
}

// Tasks embodies the pre and post conditions for the Tasks call.
type TasksCond struct {
	Store    *TaskStore
	PreOpen  bool
	ArgIDs   []int64
	RetTasks []*Task
}

func NewTasksCond(ids []int64) *TasksCond {
	return &TasksCond{
		ArgIDs: ids,
	}
}

func (c *TasksCond) Pre(info interface{}) bool {
	c.Store = info.(*TaskStore)
	c.PreOpen = c.Store.IsOpen()
	return true
}

func (c *TasksCond) Call() {
	c.RetTasks = c.Store.Tasks(c.ArgIDs)
}

func (c *TasksCond) Post() bool {
	if !c.PreOpen {
		if c.RetTasks != nil {
			fmt.Println("Tasks Postcondition: non-nil tasks on closed store.")
			return false
		}
		return true
	}
	if len(c.RetTasks) > len(c.ArgIDs) {
		fmt.Println("Tasks Postcondition: more tasks returned than requested.")
		return false
	}
	idmap := make(map[int64]struct{})
	for _, id := range c.ArgIDs {
		idmap[id] = struct{}{}
	}
	for i, t := range c.RetTasks {
		if t != nil {
			if _, ok := idmap[t.ID]; !ok {
				fmt.Printf("Tasks Postcondition: returned task %d not in requested ID list %d.\n", t.ID, c.ArgIDs)
				return false
			}
			if t.ID != c.ArgIDs[i] {
				fmt.Printf("Tasks Postcondition: returned task %d not expected task %d.\n", t.ID, c.ArgIDs[i])
				return false
			}
		}
		delete(idmap, c.ArgIDs[i])
	}
	if len(idmap) != 0 {
		fmt.Printf("Tasks Postcondition: not all tasks accounted for. missing %v.\n", idmap)
		return false
	}
	return true
}

// UpdateCond embodies the pre and post conditions for the Update call.
type UpdateCond struct {
	Store     *TaskStore
	PreOpen   bool
	PreChange []*Task
	PreDelete []*Task
	PreDepend []*Task
	ArgOwner  int32
	ArgAdd    []*Task
	ArgChange []*Task
	ArgDelete []int64
	ArgDepend []int64
	RetTasks  []*Task
	RetErr    error
	Now       int64
}

func NewUpdateCond(owner int32, add, change []*Task, del, dep []int64) *UpdateCond {
	return &UpdateCond{
		ArgOwner:  owner,
		ArgAdd:    add,
		ArgChange: change,
		ArgDelete: del,
		ArgDepend: dep,
	}
}

func (c *UpdateCond) Pre(info interface{}) bool {
	c.Store = info.(*TaskStore)
	changeIDs := make([]int64, len(c.ArgChange))
	for i, t := range c.ArgChange {
		changeIDs[i] = t.ID
	}
	c.PreOpen = c.Store.IsOpen()
	if c.PreOpen {
		c.PreChange = c.Store.Tasks(changeIDs)
		c.PreDelete = c.Store.Tasks(c.ArgDelete)
		c.PreDepend = c.Store.Tasks(c.ArgDepend)
	}
	return true
}

func (c *UpdateCond) Call() {
	c.Now = NowMillis()
	c.RetTasks, c.RetErr = c.Store.Update(c.ArgOwner, c.ArgAdd, c.ArgChange, c.ArgDelete, c.ArgDepend)
}

func (c *UpdateCond) Post() bool {
	if !c.PreOpen {
		if c.RetErr == nil {
			fmt.Println("Update Postcondition: no error returned when updating a closed store.")
			return false
		}
		return true
	}

	if c.RetErr != nil {
		if c.existMissingDependencies() {
			// We expect an error if dependencies are missing.
			return true
		}
		if c.existAlreadyOwned() {
			// We expect an error if changes or deletions are owned elsewhere.
			return true
		}
		fmt.Printf("Update Postcondition: all tasks exist, none are owned by others, but still got an error: %v\n", c.RetErr)
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

	if len(c.RetTasks) != len(c.ArgAdd)+len(c.ArgChange) {
		fmt.Printf("Update Postcondition: no error returned, but returned tasks not equal to sum of additions and changes: %v != %v + %v\n",
			len(c.RetTasks), len(c.ArgAdd), len(c.ArgChange))
		return false
	}

	newAdds := c.RetTasks[:len(c.ArgAdd)]
	for i, toAdd := range c.ArgAdd {
		added := newAdds[i]
		if added.OwnerID != c.ArgOwner {
			fmt.Printf("Update PostCondition: added task does not have the proper owner set: expected %v, got %v\n", c.ArgOwner, added.OwnerID)
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
			expectedAT = c.Now - toAdd.AT
		}
		if expectedAT > added.AT+5000 || expectedAT < added.AT-5000 {
			fmt.Printf("Update Postcondition: added task has weird AT: expected\n%v\ngot\n%v\n", toAdd, added)
			return false
		}
	}
	newChanges := c.RetTasks[len(c.ArgAdd):]
	for i, toChange := range c.ArgChange {
		changed := newChanges[i]
		if changed.OwnerID != c.ArgOwner {
			fmt.Printf("Update Postcondition: changed task does not have the proper owner set: expected %v, got %v\n", c.ArgOwner, changed.OwnerID)
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
		oldIDs := make([]int64, len(c.ArgChange))
		for i, t := range c.ArgChange {
			oldIDs[i] = t.ID
		}
		oldTasks := c.Store.Tasks(oldIDs)
		for _, t := range oldTasks {
			if t != nil {
				fmt.Printf("Update Postcondition: changed a task, but the old task is still present in the task store: %v\n", t)
				return false
			}
		}
	}

	deleted := c.Store.Tasks(c.ArgDelete)
	for i, t := range deleted {
		if t != nil {
			fmt.Printf("Update Postcondition: expected deleted tasks to disapper, but found %d still there.\n",
				c.ArgDelete[i])
			return false
		}
	}

	groups := c.Store.Groups()
	groupMap := make(map[string]struct{})
	for _, g := range groups {
		groupMap[g] = struct{}{}
	}
	for _, t := range c.ArgAdd {
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
	for _, t := range c.PreChange {
		if t.OwnerID != c.ArgOwner {
			return true
		}
	}
	for _, t := range c.PreDelete {
		if t.OwnerID != c.ArgOwner {
			return true
		}
	}
	return false
}

func (c *UpdateCond) existMissingDependencies() bool {
	for _, t := range c.PreChange {
		if t == nil {
			return true
		}
	}
	for _, t := range c.PreDelete {
		if t == nil {
			return true
		}
	}
	for _, t := range c.PreDepend {
		if t == nil {
			return true
		}
	}
	return false
}

// OpenCond embodies the pre and post conditions for the Open{Opportunistic,Strict} call.
type OpenCond struct {
	Strict     bool
	PreBusy    bool
	ArgJournal journal.Interface
	RetStore   *TaskStore
	RetErr     error
}

func NewOpenCond(journal journal.Interface, strict bool) *OpenCond {
	return &OpenCond{
		ArgJournal: journal,
		Strict:     strict,
	}
}

func (c *OpenCond) Store() *TaskStore {
	return c.RetStore
}

func (c *OpenCond) Pre(info interface{}) bool {
	if reflect.ValueOf(c.ArgJournal).IsNil() {
		fmt.Printf("Open Precondition: nil journal: %v\n", c.ArgJournal)
		return false
	}
	return true
}

func (c *OpenCond) Call() {
	if c.Strict {
		c.RetStore, c.RetErr = OpenStrict(c.ArgJournal)
	} else {
		c.RetStore, c.RetErr = OpenOpportunistic(c.ArgJournal)
	}
}

func (c *OpenCond) Post() bool {
	if c.RetErr != nil {
		if c.RetStore != nil {
			fmt.Printf("Open Postcondition: error returned, but store exists: %v\n", c.RetErr)
			return false
		}
		return true
	}

	if !c.RetStore.IsOpen() {
		fmt.Printf("Open Postcondition: successful open, but store is not open: %v\n", c.RetStore)
		return false
	}
	if c.RetStore.IsStrict() != c.Strict {
		fmt.Printf("Open Postcondition: successful open, but strict (%v) should be %v: %v\n",
			c.RetStore.IsStrict(), c.Strict, c.RetStore)
		return false
	}
	// TODO: it would be nice to check that the journal and the contents of the
	// store actually match. But this is a lot more work, so we'll skip it for
	// now.
	return true
}
