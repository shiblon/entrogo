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

package journal

import (
	"bytes"
	"io"
	"os"
)

type OS interface {
	Create(name string) (File, error)
	Open(name string) (File, error)
	Rename(oldname, newname string) error
}

type File interface {
	io.ReadWriteCloser
	Sync() error
}

type OSOS struct{}

func (OSOS) Open(name string) (File, error) {
	return os.Open(name)
}

func (OSOS) Create(name string) (File, error) {
	return os.Create(name)
}

func (OSOS) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

type memFile struct {
	bytes.Buffer
	name  string
	open  bool
}

func (f *memFile) Close() error {
	f.open = false
}

type MemOS struct {
	files map[string]*memFile
}

func (m *MemOS) Create(name string) (File, error) {
	if _, ok := m.files[name] {
		return nil, &os.PathError{Op: "Create", Path: name, Err: os.ErrExist}
	}
	f := &memFile{name: name, open: true}
	m.files[name] = f
	return f, nil
}

func (m *MemOS) Open(name string) (File, error) {
	f, ok := m.files[name]
	if !ok {
		return nil, &os.PathError{Op: "Open", Path: name, Err: os.ErrNotExist}
	}
	f.open = true
	return f, nil
}

func (m *MemOS) Rename(oldname, newname string) error {
	f, ok := m.files[name]
	if !ok {
		return &os.PathError{Op: "Rename", Path: oldname, Err: os.ErrNotExist}
	}
	_, ok = m.files[name]
	if ok {
		return &os.PathError{Op: "Rename", Path: newname, Err: os.ErrExist}
	}
	f.name = newname
	m.files[newname] = f
	delete(m.files, oldname)
	return nil
}

func (m *MemOS) Sync() error {
	return nil
}
