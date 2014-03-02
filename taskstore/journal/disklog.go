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
	"io"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var (
	FileNameRE = regexp.MustCompile(`^(\w+)\.(\d+)(?:\.(\w+))?$`)
)

const (
	LogFilePrefix = "log"
	SnapshotFilePrefix = "snapshot"
)

type DiskLog struct {
	dir string
	log *logFile

	rot  chan chan error
	quit chan bool
	add  chan interface{}
}

type addRequest struct {
	val interface{}
	resp chan error
}

type snapRequest struct {
	elems <-chan interface{}
	snapresp chan<- error
	resp chan error
}

func NewDiskLog(dir string) *DiskLog {
	d := &DiskLog{
		dir:  dir,
		add:  make(chan addRequest, 1),
		rot:  make(chan chan error, 1),
		snap: make(chan snapRequest, 1),
		quit: make(chan bool, 1),
	}
	go func() {
		for {
			select {
			case req := <-d.add:
				req.resp <- d.log.Add(req.rec)
			case resp := <-d.rot:
				resp <- d.rotateLog()
			case req := <-d.snap:
				req.resp <- d.snapshot(req.elems, req.snapresp)
			case <-d.quit:
				return
			}
		}
	}()
	return d
}

func (d *DiskLog) snapshot(elems <-chan interface{}, resp chan<- error) error {
	lastname := d.log.Name()
	lastid := d.log.ID()
	if err := d.rotateLog(); err != nil {
		return err
	}
	// Once the rotation is complete, we try to create a file (still
	// synchronous) and then kick off an asynchronous snapshot process.
	snapname := makeName(d.dir, SnapshotFilePrefix, lastid, "inprogress")
	donename := makeName(d.dir, SnapshotFilePrefix, lastid, "done")
	file, err := os.Create(snapname)
	if err != nil {
		return err
	}
	encoder := gob.Encoder(file)
	go func() {
		defer file.Close()
		defer func() {
			num := 0
			for _ := range elems {
				num++
			}
			log.Printf("consumed but did not snapshot %d elements", num)
		}() // make sure we consume all of them.

		for elem := range elems {
			if err := encoder.Encode(elem); err != nil {
				resp <- fmt.Errorf("snapshot failed to encode element %#v: #v", elem, err)
				return
			}
		}
		file.Close()

		// Now we indicate that the file is finished by renaming it.
		if err := os.Rename(snapname, donename); err != nil {
			resp <- fmt.Errorf("snapshot incomplete, failed to rename %q to %q: %v", snapname, donename, err)
			return
		}

		// Finally, we delete all of the journal files that participated up to this point.
		doneglob := filepath.Join(f.dir, "*")
		names, err := filepath.Glob(doneglob)
		if err != nil {
			resp <- fmt.Errorf("snapshot failed to match files with %q: %v", doneglob, err)
			return
		}
		// Mark all previous journals, finished or otherwise, as obsolete.
		for name := range names {
			_, prefix, birth, suffix, err := parseName(name)
			// Non-matching files are ignored.
			if err != nil {
				fmt.Printf("weird file name %q found - ignoring: %v\n", name, err)
				continue
			}
			// Skip non-log files and files that are newer than the snapshot.
			if prefix != LogFilePrefix || birth.Unix() > lastid {
				continue
			}
			// Finally, rename this log file to an obsolete name so that it can be cleaned up later.
			obsname := fmt.Sprintf("%s.obsolete", name)
			if err := os.Rename(name, obsname) {
				fmt.Printf("failed to rename %q to %q: %v\n", name, obsname, err)
				continue
			}
		}
		resp <- nil
	}()
	return nil
}

func (d *DiskLog) rotateLog() error {
	if err := d.log.Close(); err != nil {
		return fmt.Errorf("failed to close old log: %v", err)
	}
	newname := fmt.Sprintf("%s.done", d.log.Name())
	if err := os.Rename(d.log.Name(), newname); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %v", d.log.Name(), newname, err)
	}
	log, err := NewLogFile(d.dir)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	f.log = log
	return nil
}

func (d *DiskLog) Dir() string {
	return d.dir
}

func (d *DiskLog) AppendRecord(rec interface{}) error {
	resp := make(chan error, 1)
	d.add <- addRequest{
		rec,
		resp,
	}
	return <-resp
}

// Snapshot triggers an immediate rotation, then consumes all of the elements
// on the channel and serializing them to a snapshot file with the same ID as
// the recently-closed log.
func (d *DiskLog) Snapshot(elems <-chan interface{}, snapresp chan<- error) error {
	resp := make(chan error, 1)
	d.snap <- snapRequest{
		elems,
		snapresp,
		resp,
	}
	return <-resp
}

// Rotate closes the current log file and opens a new one.
func (d *DiskLog) Rotate() error {
	resp := make(chan error, 1)
	d.rot <- resp
	return <-resp
}

func makeName(dir, prefix string, birth time.Time, suffix string) string {
	name := fmt.Sprintf("%s.%d", prefix, birth.Unix())
	if suffix != "" {
		name = fmt.Sprintf("%s.%s", name, suffix)
	}
	return filepath.Join(dir, name)
}

func parseName(name string) (dir, prefix string, birth time.Time, suffix string, err error) {
	base := filepath.Base(name)
	if base == "" {
		err = fmt.Errorf("invalid file name: %s", base)
		return
	}
	parts := FileNameRE.FindStringSubmatch(base)
	if parts == nil {
		err = fmt.Errorf("Cannot parse %q into a usable name", base)
		return
	}
	num, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		err = fmt.Errorf("Invalid file id in %q: %s", base, parts[2])
		return
	}
	return filepath.Dir(name), parts[1], time.Unix(num, 0), parts[3], nil
}


type LogFile struct {
	dir string
	birth  time.Time

	file    *os.File
	encoder *gob.Encoder
}

func NewLogFile(dir string) (*LogFile, error) {
	f := &LogFile{
		dir: dir,
		birth: time.Now(),
	}
	var err error
	f.file, err = os.Create(f.Name())
	if err != nil {
		return nil, err
	}
	f.encoder = gob.NewEncoder(f.file)
	return f, nil
}

func NewLogFileName(path string) (*LogFile, error) {
	dir, prefix, birth, suffix, err := parseName(name)
	if err != nil {
		return nil, err
	}
	if prefix != LogFilePrefix {
		return nil, fmt.Errorf("invalid log file prefix %q", prefix)
	}
	if suffix != "" {
		return nil, fmt.Errorf("can't open finished logfile %s", path)
	}
	f := &LogFile{
		dir: dir,
		birth: birth,
	}
	// No suffix - we can write to this.
	file, err := os.OpenFile(f.Name(), os.O_APPEND | os.O_CREATE | os.O_SYNC, 0660)
	if err != nil {
		return nil, err
	}
	f.file = file
	f.encoder = gob.NewEncoder(f.file)
	return f
}

func (f *LogFile) ID() int64 {
	return birth.Unix()
}

func (f *LogFile) Name() string {
	return makeName(f.dir, LogFilePrefix, f.birth)
}

func (f *LogFile) Prefix() string {
	return f.prefix
}

func (f *logfile) Age() time.Duration {
	return f.age
}

func (f *logFile) Size() int64 {
	info, err := f.file.Stat()
	if err != nil {
		return -1
	}
	return info.Size()
}

func (f *logFile) Close() error {
	err := f.file.Close()
	f.file = nil
	f.encoder = nil

	return f.file.Close()
}

func (f *logFile) Add(rec interface{}) error {
	if err := f.encoder.Encode(rec); err != nil {
		return err
	}
	if err := f.file.Sync(); err != nil {
		return err
	}
	return nil
}
