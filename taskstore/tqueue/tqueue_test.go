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

package tqueue

import (
	"fmt"
)

var (
	BasicTasks = []*Task{
		{
			ID:    1,
			AT:    1000,
			Owner: 1,
			Group: "group1",
			Data:  "data1",
		},
		{
			ID:    2,
			AT:    1004,
			Owner: 1,
			Group: "group1",
			Data:  "data2",
		},
		{
			ID:    3,
			AT:    999,
			Owner: 1,
			Group: "group1",
			Data:  "data3",
		},
		{
			ID:    4,
			AT:    1005,
			Owner: 1,
			Group: "group1",
			Data:  "data4",
		},
		{
			ID:    5,
			AT:    1002,
			Owner: 1,
			Group: "group1",
			Data:  "data5",
		},
		{
			ID:    6,
			AT:    1001,
			Owner: 1,
			Group: "group1",
			Data:  "data6",
		},
		{
			ID:    7,
			AT:    1003,
			Owner: 1,
			Group: "group1",
			Data:  "data7",
		},
	}
)

func Example_new() {
	tq := New("group1")
	fmt.Println(tq.name, tq.taskHeap, cap(tq.randChan))

	// Output:
	// group1 [] 1
}

func Example_newFromTasks() {
	BasicTasks = []*Task{
		{
			ID:    1,
			AT:    1000,
			Owner: 1,
			Group: "group1",
			Data:  "data1",
		},
		{
			ID:    2,
			AT:    1004,
			Owner: 1,
			Group: "group1",
			Data:  "data2",
		},
		{
			ID:    3,
			AT:    999,
			Owner: 1,
			Group: "group1",
			Data:  "data3",
		},
		{
			ID:    4,
			AT:    1005,
			Owner: 1,
			Group: "group1",
			Data:  "data4",
		},
		{
			ID:    5,
			AT:    1002,
			Owner: 1,
			Group: "group1",
			Data:  "data5",
		},
		{
			ID:    6,
			AT:    1001,
			Owner: 1,
			Group: "group1",
			Data:  "data6",
		},
		{
			ID:    7,
			AT:    1003,
			Owner: 1,
			Group: "group1",
			Data:  "data7",
		},
	}

	tq := NewFromTasks("group1", BasicTasks)

	fmt.Println(tq)

	// Output:
	//
	// TQ name=group1
	//    heap=[
	//       {0:Task 3: group=group1 owner=1 at=999 data="data3"}
	//       {1:Task 5: group=group1 owner=1 at=1002 data="data5"}
	//       {2:Task 1: group=group1 owner=1 at=1000 data="data1"}
	//       {3:Task 4: group=group1 owner=1 at=1005 data="data4"}
	//       {4:Task 2: group=group1 owner=1 at=1004 data="data2"}
	//       {5:Task 6: group=group1 owner=1 at=1001 data="data6"}
	//       {6:Task 7: group=group1 owner=1 at=1003 data="data7"}
	//    ]
	//    chancap=1
}

func ExampleTQueue_Push() {
	tq := NewFromTasks("group1", BasicTasks)
	tq.Push(&Task{
		ID:    7,
		AT:    998,
		Group: "group1",
		Data:  nil,
	})

	fmt.Println(tq)

	// Output:
	//
	// TQ name=group1
	//    heap=[
	//       {0:Task 7: group=group1 owner=0 at=998 data=<nil>}
	//       {1:Task 3: group=group1 owner=1 at=999 data="data3"}
	//       {2:Task 1: group=group1 owner=1 at=1000 data="data1"}
	//       {3:Task 5: group=group1 owner=1 at=1002 data="data5"}
	//       {4:Task 2: group=group1 owner=1 at=1004 data="data2"}
	//       {5:Task 6: group=group1 owner=1 at=1001 data="data6"}
	//       {6:Task 7: group=group1 owner=1 at=1003 data="data7"}
	//       {7:Task 4: group=group1 owner=1 at=1005 data="data4"}
	//    ]
	//    chancap=1
}

func ExampleTQueue_Pop() {
	tq := NewFromTasks("group1", BasicTasks)

	// Pop the oldest task (the lowest Available Time)
	task := tq.Pop()

	fmt.Println(task)

	// Output:
	// Task 3: group=group1 owner=1 at=999 data="data3"
}

func ExampleTQueue_PopAt() {
	tq := NewFromTasks("group1", BasicTasks)

	// Pop a task from the middle
	task := tq.PopAt(4)

	// Note that we get the proper task, and the queue is reorganized to fit
	// the new situation.
	fmt.Println(task)
	fmt.Println(tq)

	// Output:
	// Task 2: group=group1 owner=1 at=1004 data="data2"
	// TQ name=group1
	//    heap=[
	//       {0:Task 3: group=group1 owner=1 at=999 data="data3"}
	//       {1:Task 5: group=group1 owner=1 at=1002 data="data5"}
	//       {2:Task 1: group=group1 owner=1 at=1000 data="data1"}
	//       {3:Task 4: group=group1 owner=1 at=1005 data="data4"}
	//       {4:Task 7: group=group1 owner=1 at=1003 data="data7"}
	//       {5:Task 6: group=group1 owner=1 at=1001 data="data6"}
	//    ]
	//    chancap=1
}

func ExampleTQueue_Peek() {
	tq := NewFromTasks("group1", BasicTasks)

	fmt.Println(tq.Peek())

	// Output:
	// Task 3: group=group1 owner=1 at=999 data="data3"
}

func ExampleTQueue_PeekAt() {
	tq := NewFromTasks("group1", BasicTasks)

	fmt.Println(tq.PeekAt(3))

	// Output:
	// Task 4: group=group1 owner=1 at=1005 data="data4"
}

func ExampleTQueue_PopRandomAvailable_onlyOne() {
	tq := NewFromTasks("group1", BasicTasks)
	task := tq.PopRandomAvailable(999) // Only one matches.
	fmt.Println(task)

	// Output:
	//
	// Task 3: group=group1 owner=1 at=999 data="data3"
}

func ExampleTQueue_PopRandomAvailable_none() {
	tq := NewFromTasks("group1", BasicTasks)
	task := tq.PopRandomAvailable(998)
	fmt.Println(task == nil)

	// Output:
	//
	// true
}

func ExampleTQueue_PopRandomAvailable_random() {
	tq := NewFromTasks("group1", BasicTasks)
	task := tq.PopRandomAvailable(1001)

	switch task.ID {
	case 1, 3, 6:
		fmt.Println("Yes")
	default:
		fmt.Println("No")
	}

	// Output:
	//
	// Yes
}
