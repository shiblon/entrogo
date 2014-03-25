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

// A RESTful HTTP-based task service that uses the taskstore.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"code.google.com/p/entrogo/taskstore"
	"code.google.com/p/entrogo/taskstore/journal"
)

var (
	jdir            = flag.String("jdir", "", "directory to hold the task store journal - only one process should ever access it at a time.")
	port            = flag.Int("port", 8048, "port on which to listen for task requests")
	isOpportunistic = flag.Bool("opp", false, "turns on opportunistic journaling when true. This means that task updates are flushed to disk when possible. Leaving it strict means that task updates are flushed before given back to the caller.")
)

type TaskInfo struct {
	ID    int64  `json:'id'`
	Group string `json:'group'`
	Data  string `json:'data'`

	// The TimeSpec, when positive, indicates an absolute timestamp in
	// milliseconds since the epoch (UTC). When negative, its absolute value
	// will be added to the current time to create an appropriate timestamp.
	TimeSpec int64 `json:'duration'`
}

type UpdateRequest struct {
	ClientID int32      `json:'clientid'`
	Adds     []TaskInfo `json:'adds'`
	Updates  []TaskInfo `json:'updates'`
	Deletes  []int64    `json:'deletes'`
	Depends  []int64    `json:'depends'`
}

type UpdateResponse struct {
	Success   bool       `json:'success'`
	NewTasks  []TaskInfo `json:'newtasks'`
	Errors    []string   `json:'errors'`
}

type ClaimRequest struct {
	ClientID int32   `json:'clientid'`
	Group    string  `json:'group'`
	Limit    int32   `json:'limit'`
	Duration int64   `json:'duration'`
	Depends  []int64 `json:'depends'`
}

type ClaimResponse struct {
	Success bool     `json:'success'`
	Task    TaskInfo `json:'task'`
	Errors  []string `json:'errors'`
}

type HandlerStore struct {
	store *taskstore.TaskStore
}

func NewHandlerStore(dir string, opportunistic bool) (*HandlerStore, error) {
	journaler, err := journal.NewDiskLog(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to create journal at %q: %v", dir, err)
	}

	newstoreFunc := taskstore.NewStrict
	if opportunistic {
		newstoreFunc = taskstore.NewOpportunistic
	}
	store := newstoreFunc(journaler)

	return &HandlerStore{store}, nil
}

// Groups returns a list of known groups in the task store.
func (s *HandlerStore) Groups(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "GET method required for group listing")
		return
	}
	groups := s.store.Groups()
	out, err := json.Marshal(groups)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("Error forming json: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

// Tasks returns a list of tasks for the specified group. It can be limited
// to a certain number, and can optionally allow owned tasks to be returned as
// well as unowned.
func (s *HandlerStore) Tasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.getTasks(w, r)
	case "PUT":
		s.putTasks(w, r)
	case "POST":
		w.WriteHeader(http.StatusMethodNotAllowed)
	case "DELETE":
		w.WriteHeader(http.StatusMethodNotAllowed)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Claim accepts a group name with optional limit, duration, and dependencies.
func (s *HandlerStore) Claim(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.WriteHeader(http.StatusMethodNotAllowed)
	case "PUT":
		s.putClaim(w, r)
	case "POST":
		w.WriteHeader(http.StatusMethodNotAllowed)
	case "DELETE":
		w.WriteHeader(http.StatusMethodNotAllowed)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *HandlerStore) getTasks(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	pieces := strings.Split(r.URL.Path, "/")
	if len(pieces) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("invalid tasks request, expected /tasks/<groupname>, got %v\n", pieces))
		return
	}
	name := pieces[1]

	var err error
	limit := 0
	allowOwned := false
	if lstr := r.Form.Get("limit"); lstr != "" {
		limit, err = strconv.Atoi(lstr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, fmt.Sprintf("invalid limit parameter %q, expected a number\n", lstr))
			return
		}
	}
	if astr := r.Form.Get("owned"); astr != "" {
		switch strings.ToLower(astr) {
		case "yes", "true", "1":
			allowOwned = true
		case "no", "false", "0":
			allowOwned = false
		default:
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, fmt.Sprintf("invalid owned parameter %q, expected a boolean\n", astr))
			return
		}
	}
	tasks := s.store.ListGroup(name, limit, allowOwned)
	out, err := json.Marshal(tasks)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("failed json encoding of task list: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

// putClaim is called when a task is to be claimed.
func (s *HandlerStore) putClaim(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	claimstr := r.Form.Get("claim")
	if claimstr == "" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "no parameters provided for task claim")
		return
	}

	var claim ClaimRequest
	err := json.Unmarshal([]byte(claimstr), claim)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("failed to decode json claim %v: %q", claimstr, err))
		return
	}

	task, err := s.store.Claim(claim.ClientID, claim.Group, claim.Duration, claim.Depends)
	if err != nil {
		uerr := err.(taskstore.UpdateError)
		estrs := make([]string, len(uerr.Errors))
		for i, e := range uerr.Errors {
			estrs[i] = e.Error()
		}

		response := ClaimResponse{
			Success: false,
			Errors: estrs,
		}
		out, jerr := json.Marshal(response)
		if jerr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, fmt.Sprintf("failed to marshal failed claim response: %v", jerr))
			return
		}
		w.Write(out)
		return
	}
	response := ClaimResponse{
		Success: true,
		Task: TaskInfo{
			ID: task.ID,
			Group: task.Group,
			Data: string(task.Data),
			TimeSpec: task.AvailableTime,
		},
	}
	out, jerr := json.Marshal(response)
	if jerr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("failed to marshal successful claim response: %v", jerr))
		return
	}
	w.Write(out)
	return
}

// putTasks is called when a task updated is attempted. It calls taskstore.TaskStore.Update.
func (s *HandlerStore) putTasks(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	updatestr := r.Form.Get("update")
	if updatestr == "" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "no data provided for task update")
		return
	}

	var update UpdateRequest
	err := json.Unmarshal([]byte(updatestr), update)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("failed to decode json update %v: %v", updatestr, err))
		return
	}

	// We have an update request. Reformat to proper tasks, etc., as expected by the taskstore.
	adds := make([]*taskstore.Task, len(update.Adds))
	updates := make([]*taskstore.Task, len(update.Updates))

	now := taskstore.NowMillis()

	for i, a := range update.Adds {
		ts := a.TimeSpec
		if ts <= 0 {
			ts = now - ts
		}
		adds[i] = &taskstore.Task{
			ID:            0,
			Group:         a.Group,
			AvailableTime: ts,
			Data:          []byte(a.Data),
		}
	}

	for i, u := range update.Updates {
		ts := u.TimeSpec
		if ts <= 0 {
			ts = now - ts
		}
		updates[i] = &taskstore.Task{
			ID:            u.ID,
			Group:         u.Group,
			AvailableTime: ts,
			Data:          []byte(u.Data),
		}
	}

	// Perform the actual update. Finally.
	newtasks, err := s.store.Update(update.ClientID, adds, updates, update.Deletes, update.Depends)
	if err != nil {
		out, jerr := json.Marshal(err)
		if jerr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, fmt.Sprintf("failed to marshal the json encoding error that follows: %v\n%v", jerr, err))
			return
		}
		// update errors are fine and expected. We just return an error object in that case.
		uerr := err.(taskstore.UpdateError)
		estrs := make([]string, len(uerr.Errors))
		for i, e := range uerr.Errors {
			estrs[i] = e.Error()
		}
		response := UpdateResponse{
			Success:   false,
			Errors:    estrs,
		}
		out, jerr = json.Marshal(response)
		if jerr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, fmt.Sprintf("failed to marshal failed update response: %v", jerr))
			return
		}
		w.Write(out)
		return
	}

	outtasks := make([]TaskInfo, len(newtasks))
	for i, t := range newtasks {
		outtasks[i] = TaskInfo{
			ID:       t.ID,
			Group:    t.Group,
			Data:     string(t.Data),
			TimeSpec: t.AvailableTime,
		}
	}

	response := UpdateResponse{
		Success:   true,
		NewTasks:  outtasks,
	}
	out, jerr := json.Marshal(response)
	if jerr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("failed to marshal successful update response %v", jerr))
		return
	}

	w.Write(out)
}

func main() {
	flag.Parse()

	if *jdir == "" {
		fmt.Println("please specify a journal directory via -jdir")
		os.Exit(-1)
	}

	store, err := NewHandlerStore(*jdir, *isOpportunistic)
	if err != nil {
		fmt.Println("failed to create a task store: %v", err)
		os.Exit(-1)
	}

	http.HandleFunc("/test/", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, fmt.Sprintf("%s\n", r.URL.Path))
		io.WriteString(w, fmt.Sprintf("%#v\n", r.Form))
		io.WriteString(w, fmt.Sprintf("Test\n"))
	})
	http.HandleFunc("/groups", store.Groups)
	http.HandleFunc("/tasks", store.Tasks)
	http.HandleFunc("/tasks/", store.Tasks) // GET takes a required group name
	http.HandleFunc("/claim/", store.Claim) // PUT takes a required group name

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
