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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

type FS interface {
	Create(name string) (File, error)
	Open(name string) (File, error)
	Rename(oldname, newname string) error
	Remove(name string) error
	Lock(name string) error
	Unlock(name string) error
	Stat(name string) (os.FileInfo, error)
	FindMatching(glob string) ([]string, error)
}

type File interface {
	io.ReadWriteCloser
	Name() string
	Sync() error
}

type OSFS struct{}

func (OSFS) Open(name string) (File, error) {
	return os.Open(name)
}

func (OSFS) Create(name string) (File, error) {
	return os.Create(name)
}

func (OSFS) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

func (OSFS) Remove(name string) error {
	return os.Remove(name)
}

func (OSFS) Lock(name string) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer func() { f.Close() }()
	// non-blocking exclusive lock
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		os.Remove(name)
		return err
	}
	f.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
	return nil
}

func (OSFS) Unlock(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(name)
	}()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return err
	}
	if err := os.Remove(name); err != nil {
		return err
	}
	return nil
}

func (OSFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (OSFS) FindMatching(glob string) ([]string, error) {
	return filepath.Glob(glob)
}

type memFile struct {
	bytes.Buffer
	name    string
	open    bool
	modtime time.Time
	isdir   bool
}

func (f *memFile) Close() error {
	f.open = false
	return nil
}

func (f *memFile) Sync() error {
	return nil
}

func (f *memFile) Name() string {
	return f.name
}

type memFileInfo struct {
	name    string
	size    int64
	modtime time.Time
	isdir   bool
}

func (fi *memFileInfo) Name() string {
	return fi.name
}

func (fi *memFileInfo) Size() int64 {
	return fi.size
}

func (fi *memFileInfo) Mode() os.FileMode {
	return 0666
}

func (fi *memFileInfo) ModTime() time.Time {
	return fi.modtime
}

func (fi *memFileInfo) IsDir() bool {
	return fi.isdir
}

func (fi *memFileInfo) Sys() interface{} {
	return nil
}

type MemFS struct {
	lockmx sync.Mutex
	files  map[string]*memFile
}

func NewMemFS(dirs ...string) *MemFS {
	m := &MemFS{
		files: make(map[string]*memFile),
	}
	now := time.Now()
	for _, d := range dirs {
		m.files[d] = &memFile{name: d, modtime: now, isdir: true}
	}
	return m
}

func (m *MemFS) Create(name string) (File, error) {
	if _, ok := m.files[name]; ok {
		return nil, &os.PathError{Op: "Create", Path: name, Err: os.ErrExist}
	}
	f := &memFile{name: name, open: true, modtime: time.Now()}
	m.files[name] = f
	return f, nil
}

func (m *MemFS) Open(name string) (File, error) {
	f, ok := m.files[name]
	if !ok {
		return nil, &os.PathError{Op: "Open", Path: name, Err: os.ErrNotExist}
	}
	f.open = true
	return f, nil
}

func (m *MemFS) Rename(oldname, newname string) error {
	f, ok := m.files[oldname]
	if !ok {
		return &os.PathError{Op: "Rename", Path: oldname, Err: os.ErrNotExist}
	}
	_, ok = m.files[newname]
	if ok {
		return &os.PathError{Op: "Rename", Path: newname, Err: os.ErrExist}
	}
	f.name = newname
	m.files[newname] = f
	delete(m.files, oldname)
	return nil
}

func (m *MemFS) Remove(name string) error {
	delete(m.files, name)
	return nil
}

func (m *MemFS) Lock(name string) error {
	defer un(lock(&m.lockmx))

	if _, ok := m.files[name]; ok {
		return os.ErrExist
	}
	m.Create(name)
	return nil
}

func (m *MemFS) Unlock(name string) error {
	defer un(lock(&m.lockmx))

	if _, ok := m.files[name]; !ok {
		return os.ErrNotExist
	}
	m.Remove(name)
	return nil
}

func (m *MemFS) Stat(name string) (os.FileInfo, error) {
	if f, ok := m.files[name]; ok {
		return &memFileInfo{
			name:    f.name,
			size:    int64(f.Len()),
			modtime: f.modtime,
			isdir:   f.isdir,
		}, nil
	}
	return nil, &os.PathError{Op: "Stat", Path: name, Err: os.ErrNotExist}
}

func (m *MemFS) FindMatching(glob string) ([]string, error) {
	var matches []string
	var names []string
	for name := range m.files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		matched, err := filepath.Match(glob, name)
		if err != nil {
			return matches, err
		}
		if matched {
			matches = append(matches, name)
		}
	}
	return matches, nil
}

func (m *MemFS) String() string {
	lines := []string{fmt.Sprintf("MemFS %p", m)}
	names := make([]string, 0, len(m.files))
	for k := range m.files {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		s, err := m.Stat(n)
		if err != nil {
			lines = append(lines, fmt.Sprintf("  %s: <stat err %v>", n, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s: %d", n, s.Size()))
	}
	return strings.Join(lines, "\n")
}
