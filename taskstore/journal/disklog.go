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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DiskLog struct {
	dir string

	journalName  string
	journalFile  *os.File
	journalEnc   *gob.Encoder
	journalBirth time.Time

	rot  chan chan error
	quit chan bool
	add  chan interface{}
}

type addRequest struct {
	val  interface{}
	resp chan error
}

type snapRequest struct {
	elems    <-chan interface{}
	snapresp chan<- error
	resp     chan error
}

func NewDiskLog(dir string) *DiskLog {
	// TODO(chris): find a way to ensure this is a singleton for the given directory.
	d := &DiskLog{
		dir:  dir,
		add:  make(chan addRequest, 1),
		rot:  make(chan chan error, 1),
		snap: make(chan snapRequest, 1),
		quit: make(chan bool, 1),
	}

	// We *always* open a new log, even if there was one open when we last terminated.
	// This allows us to ignore any corrupt records at the end of the old one
	// without doing anything complicated to find out where they are, etc. Just
	// open a new log and have done with it. It's simpler and safer.
	d.openNewLog()

	go func() {
		for {
			select {
			case req := <-d.add:
				req.resp <- d.addRecord(req.rec)
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

// addRecord attempts to append the record to the end of the file, using gob encoding.
func (d *DiskLog) addRecord(rec interface{}) error {
	if err := d.journalEnc.Encode(rec); err != nil {
		return err
	}
	if err := f.journalFile.Sync(); err != nil {
		return err
	}
	return nil
}

// birthFromName gets a timestamp from the file name (it's a prefix).
func birthFromName(name string) (int64, error) {
	name = filepath.Base(name)
	pos := strings.IndexRune(name, ".")
	if pos < 0 {
		return -1, fmt.Errorf("weird name, can't find ID: %q", name)
	}
	return strconv.ParseInt(name[:pos], 10, 64)
}

// snapshot attempts to get data elements from the caller and write them all to
// a snapshot file. It always triggers a log rotation, so that any other data
// that comes in (not part of the snapshot) is strictly newer.
func (d *DiskLog) snapshot(elems <-chan interface{}, resp chan<- error) error {
	lastname := d.journalName
	lastbirth := d.journalBirth
	if err := d.rotateLog(); err != nil {
		return err
	}
	// Once the rotation is complete, we try to create a file (still
	// synchronous) and then kick off an asynchronous snapshot process.
	snapname := filepath.Join(d.dir, fmt.Sprintf("%d.%d.snapshot.working", lastbirth.Unix(), os.Getpid()))
	donename := strings.TrimSuffix(snapname, ".working")
	file, err := os.Create(snapname)
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(file)
	go func() {
		defer file.Close()
		// make sure we consume all of them to play nice with the producer.
		defer func() {
			num := 0
			for _ := range elems {
				num++
			}
			log.Printf("consumed but did not snapshot %d elements", num)
		}()

		for _, elem := range elems {
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
		doneglob := filepath.Join(f.dir, "*.*.log")
		workglob := filepath.Join(f.dir, "*.*.log.working")
		donenames, err := filepath.Glob(doneglob)
		if err != nil {
			log.Printf("finished name glob %q failed: %v", doneglob, err)
		}
		worknames, err := filepath.Glob(workglob)
		if err != nil {
			log.Printf("working name glob %q failed: %v", workglob, err)
		}
		names = make([]string, 0, len(donenames)+len(worknames))
		names = append(names, donenames...)
		names = append(names, worknames...)

		// Mark all previous journals, finished or otherwise, as obsolete.
		maxts := lastBirth.Unix()
		for _, name := range names {
			ts, err := birthFromName(name)
			if err != nil {
				log.Printf("skipping unknown name format %q: %v", name, err)
				continue
			}
			if ts > maxts {
				continue
			}

			// Finally, rename this log file to an obsolete name so that it can be cleaned up later.
			var obsname string
			if strings.HasSuffix(name, ".working") {
				obsname = fmt.Sprintf("%s.defunct", strings.TrimSuffix(name, ".working"))
			} else {
				obsname = fmt.Sprintf("%s.obsolete", name)
			}
			if err := os.Rename(name, obsname); err != nil {
				log.Printf("failed to rename %q to %q: %v\n", name, obsname, err)
				continue
			}
		}
		resp <- nil
	}()
	return nil
}

// freezeLog closes the file for the current journal, nils out the appropriate
// members, and removes the ".working" suffix from the file name.
func (d *DiskLog) freezeLog() error {
	jf := d.journalFile
	d.journalEnc = nil
	d.journalFile = nil

	if err := jf.Close(); err != nil {
		return fmt.Errorf("failed to close log: %v", err)
	}

	if !strings.HasSuffix(d.journalName, ".working") {
		return fmt.Errorf("trying to freeze an already-frozen log: %s", d.journalName)
	}
	if err := os.Rename(d.journalName, strings.TrimSuffix(d.journalName, ".working")); err != nil {
		return fmt.Errorf("failed to freeze %s by rename: %v", d.journalName, err)
	}

	return nil
}

// openExistingLog tries to open a log file that is already on the system for append.
func (d *DiskLog) openExistingLog(name string) error {
	ts, err := birthFromName(d.journalName)
	if err != nil {
		return err
	}
	birth := time.Unix(ts, 0)
	jf, err := os.OpenFile(d.journalName, os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		return err
	}
	d.journalName = name
	d.journalBirth = birth
	d.journalFile = jf
	d.journalEnc = gob.NewEncoder(jf)
	return nil
}

// openNewLog creates a new log file and sets it as the current log. It does
// not check whether another one is already open, it just abandons it without
// closing it.
func (d *DiskLog) openNewLog() error {
	birth := time.Now()
	name := filepath.Join(d.dir, fmt.Sprintf("%d.%d.log.working", birth.Unix(), os.Getpid()))
	f, err := os.OpenFile(d.journalName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	d.journalBirth = birth
	d.journalName = name
	d.journalFile = f
	d.journalEnc = gob.NewEncoder(f)
	return nil
}

// rotateLog closes and freezes the current log and opens a new one.
func (d *DiskLog) rotateLog() error {
	if err := d.freezeLog(); err != nil {
		return err
	}
	if err := d.openNewLog(); err != nil {
		return err
	}
	return nil
}

// Dir returns the file system directory for this journal.
func (d *DiskLog) Dir() string {
	return d.dir
}

// Append adds a record to the end of the journal.
func (d *DiskLog) Append(rec interface{}) error {
	resp := make(chan error, 1)
	d.add <- addRequest{
		rec,
		resp,
	}
	return <-resp
}

// StartSnapshot triggers an immediate rotation, then consumes all of the
// elements on the channel and serializing them to a snapshot file with the
// same ID as the recently-closed log.
func (d *DiskLog) StartSnapshot(elems <-chan interface{}, snapresp chan<- error) error {
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

func (d *DiskLog) latestFrozenSnapshot() (int64, string, error) {
	glob := filepath.Join(d.dir, fmt.Sprintf("*.*.snapshot")
	names, err := filepath.Glob(glob)
	if err != nil {
		return -1, "", err
	}
	if len(names) == 0 {
		return -1, "", io.EOF
	}
	bestts := -1
	bestname := ""
	for _, name := range names {
		ts := birthFromName(name)
		if ts > bestts {
			bestts = ts
			bestname = name
		}
	}
	if bestts < 0 {
		return -1, "", io.EOF
	}
	return bestts, bestname, nil
}

// SnapshotDecoder returns a decoder whose Decode function can be called to get
// the next item from the most recent frozen snapshot.
func (d *DiskLog) SnapshotDecoder() (Decoder, error) {
	_, snapname, err := d.latestFrozenSnapshot()
	if err != nil {
		return nil, err
	}

	// Found it - try to open it for reading.
	file, err := os.Open(snapname)
	if err != nil {
		return nil, err
	}
	return gob.NewDecoder(file), nil
}

type journalNames []string

func (n journalNames) Less(i, j int) bool {
	return birthFromName(n[i]) < birthFromName(n[j])
}

func (n journalNames) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n journalNames) Len() int {
	return len(n)
}

// JournalDecoder returns a Decoder whose Decode function can be called to get
// the next item from the journals that are newer than the most recent
// snapshot.
func (d *DiskLog) JournalDecoder() (Decoder, error) {
	doneglob := filepath.Join(d.dir, fmt.Sprintf("*.*.log")
	workglob := filepath.Join(d.dir, fmt.Sprintf("*.*.log.working")
	donenames, err := filepath.Glob(doneglob)
	if err != nil {
		return nil, err
	}
	worknames, err := filepath.Glob(workglob)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(donenames) + len(worknames))
	names = append(names, donenames...)
	names = append(names, worknames...)

	sort.Sort(journalNames(names))

	snapbirth, snapname, err := d.latestFrozenSnapshot()
	if err != nil && err != io.EOF {
		return nil, err
	}

	var files []*os.File
	for _, name := range names {
		if birthFromName(name) <= snapbirth {
			continue
		}
		file, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	if len(files) == 0 {
		return nil, io.EOF
	}

	return gob.NewDecoder(io.MultiReader(files...)), nil
}
