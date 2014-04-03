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

// Package client implements a client for the HTTP taskstore service.
package client

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"code.google.com/p/entrogo/taskstore/service/def"
)

var (
	clientID int32
)

func init() {
	buff := make([]byte, 4)
	n, err := crand.Read(buff)
	if err != nil {
		panic(err)
	}
	if n < len(buff) {
		panic(fmt.Sprintf("Failed to read %d bytes from crypto/rand.Reader. Only read %d bytes.", len(buff), n))
	}
	clientID = (buff[0] << 24) | (buff[1] << 16) | (buff[2] << 8) | buff[3]
}

// ID returns the (hopefully unique) ID of this client instance.
func ID() int32 {
	return clientID
}

// The HTTPError type is returned when protocol operations succeed, but non-200 responses are returned.
type HTTPError error

// An HTTPClient provides access to a particular taskstore HTTP service, as specified by a URL.
type HTTPClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPClient creates an HTTPClient that attempts to connect to the given
// base URL for all operations.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		strings.TrimSuffix(baseURL, "/"),
		&http.Client{},
	}
}

func (h *HTTPClient) doRequest(r *http.Request) (*httpResponse, error) {
	resp, err := h.client.Do(r)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error status %q: %s", resp.Status, string(ioutil.ReadAll(resp.Body)))
	}
	return resp, nil
}

// Groups retrieves a list of group names from the task service.
func (h *HTTPClient) Groups() ([]string, error) {
	req := http.NewRequest("GET", fmt.Sprintf("%s/%s", h.baseURL, "group"), nil)
	resp, err := h.doRequest(req)
	if err != nil {
		return nil, err
	}

	var groups []string
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&groups)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// Task retrieves the task for the given ID, if it exists.
func (h *HTTPClient) Task(id int64) (def.TaskInfo, error) {
	req := http.NewRequest("GET", fmt.Sprintf("%s/task/%d", h.baseURL, id), nil)
	resp, err := h.doRequest(req)
	if err != nil {
		return nil, err
	}

	var task def.TaskInfo
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&task)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// Tasks retrieves the tasks for the given list of IDs.
func (h *HTTPClient) Tasks(ids ...int64) ([]def.TaskInfo, error) {
	idstrs := make([]string, len(ids))
	for i, id := range ids {
		idstrs[i] = fmt.Sprintf("%d", id)
	}
	req := http.NewRequest("GET", fmt.Sprintf("%s/tasks/%s", h.baseURL, strings.Join(idstrs, ",")), nil)
	resp, err := h.doRequest(req)
	if err != nil {
		return nil, err
	}

	var tasks []def.TaskInfo
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&tasks)
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// Group retrieves the tasks for the given group. Optionally, a limit greater
// than zero indicates a maximum number of tasks that can be retrieved. If
// owned tasks should also be retrieved, set owned. Otherwise only tasks with
// an arrival time in the past will be returned. Note that allowing owned tasks
// does not discriminate by owner ID. All owned tasks will be allowed
// regardless of who owns them.
func (h *HTTPClient) Group(name string, limit int, owned bool) ([]def.TaskInfo, error) {
	ownint := 0
	if owned {
		ownint = 1
	}
	query := fmt.Sprintf("%s/group/%s?limit=%d&owned=%d", h.baseURL, name, limit, ownint)
	req := http.NewRequest("GET", query, nil)
	resp, err := h.doRequest(req)
	if err != nil {
		return nil, err
	}

	var tasks []def.TaskInfo
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&tasks)
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// Update attempts to add, update, and delete the specified tasks, provided
// that all dependencies are met and the operation can be completed atomically
// and by the appropriate owner, etc.
// It returns an UpdateResponse, which contains information about whether the
// operation was successful, the new tasks if any, and an error for each
// request, as appropriate.
func (h *HTTPClient) Update(adds, updates []def.TaskInfo, deletes, depends []int64) (def.UpdateResponse, error) {
	request := def.UpdateRequest{
		ClientID: ClientID(),
		Adds:     adds,
		Updates:  updates,
		Deletes:  deletes,
		Depends:  depends,
	}
	mreq, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req := http.NewRequest("POST", fmt.Sprintf("%s/update", h.baseURL), bytes.NewReader(mreq))
	resp, err := h.doRequest(req)
	if err != nil {
		return nil, err
	}

	var response def.UpdateResponse
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Claim attempts to claim a task from the given group.
// It returns a ClaimResponse.
// TODO
