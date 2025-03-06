package trcshzigfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// trcshzigfs wraps an io.ReadWriteCloser
type trcshzigFileHandle struct {
	rwc io.ReadCloser
}

type TrcshZigReadCloser struct {
	io.ReadSeeker
}

func (rwc *TrcshZigReadCloser) Close() error {
	return nil
}

type trcshZigFile struct {
	fs.Inode
	rwc io.ReadCloser
	tzr *TrcshZigRoot
}

type TrcshZigRoot struct {
	fs.Inode

	zigFiles *map[string]interface{}
	ppid     uint32
}

var _ = (fs.NodeOnAdder)((*TrcshZigRoot)(nil))

func (tzr *TrcshZigRoot) OnAdd(ctx context.Context) {
	// OnAdd is called once we are attached to an Inode. We can
	// then construct a tree.  We construct the entire tree, and
	// we don't want parts of the tree to disappear when the
	// kernel is short on memory, so we use persistent inodes.
	for path, trcshZigFileBytes := range *tzr.zigFiles {
		if path == "env" || path == "log" || path == "certify" || path == "PluginEventChannelsMap" || path == "./io/STDIO" {
			continue
		}
		dir, base := filepath.Split(path)
		if strings.Contains(path, "newrelic.yml") {
			dir = fmt.Sprintf("%snewrelic", dir)
		}

		p := &tzr.Inode
		for _, component := range strings.Split(dir, "/") {
			if len(component) == 0 || component == "." || component == "local_config" {
				continue
			}
			ch := p.GetChild(component)
			if ch == nil {
				ch = p.NewPersistentInode(ctx, &fs.Inode{},
					fs.StableAttr{Mode: fuse.S_IFDIR})
				p.AddChild(component, ch, true)
			}

			p = ch
		}

		ch := p.NewPersistentInode(ctx, NewTrcshZigFileBytes(trcshZigFileBytes.([]byte), tzr), fs.StableAttr{})
		p.AddChild(base, ch, true)
	}
}

func NewTrcshZigRoot(zigFileMap *map[string]interface{}) *TrcshZigRoot {
	return &TrcshZigRoot{zigFiles: zigFileMap}
}

func (tzr *TrcshZigRoot) SetPPid(ppid uint32) {
	tzr.ppid = ppid
}

func (tzr *TrcshZigRoot) GetPPid() uint32 {
	return tzr.ppid
}

func NewTrcshZigFileBytes(zigFileBytes []byte, tzr *TrcshZigRoot) *trcshZigFile {
	// TODO:... *zigFileBytes?
	return &trcshZigFile{rwc: &TrcshZigReadCloser{ReadSeeker: bytes.NewReader(zigFileBytes)}, tzr: tzr}
}

func NewTrcshZigFileHandle(rwc io.ReadCloser) *trcshzigFileHandle {
	return &trcshzigFileHandle{rwc: rwc}
}

func getParentPID(pid uint32) (uint32, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	statData, err := os.ReadFile(statPath)
	if err != nil {
		return 0, fmt.Errorf("no parent pid for pid: %d", pid)
	}

	fields := strings.Fields(string(statData))
	if len(fields) < 4 {
		return 0, errors.New("unreadable stat file")
	}

	ppid, err := strconv.ParseUint(fields[3], 10, 32)
	if err != nil {
		return 0, errors.New("unparsable ppid")
	}
	if ppid > math.MaxUint32 {
		return 0, errors.New("unparsable ppid")
	}

	return uint32(ppid), nil
}

func (fh *trcshZigFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	caller, ok := fuse.FromContext(ctx)
	if !ok {
		return nil, 0, syscall.EACCES
	}
	ppid, err := getParentPID(caller.Pid)
	if err != nil {
		return nil, 0, syscall.EACCES
	}

	if ppid != fh.tzr.GetPPid() {
		return nil, 0, syscall.EACCES
	}

	return NewTrcshZigFileHandle(fh.rwc), fuse.FOPEN_DIRECT_IO, 0
}

func (tzf *trcshZigFile) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if seeker, ok := tzf.rwc.(io.Seeker); ok {
		if _, err := seeker.Seek(off, io.SeekStart); err != nil {
			return nil, syscall.EIO
		}
	} else {
		return nil, syscall.ENOTSUP
	}

	n, err := tzf.rwc.Read(dest)
	if err != nil && err != io.EOF {
		return nil, syscall.EIO
	}

	return fuse.ReadResultData(dest[:n]), 0
}

func (fh *trcshzigFileHandle) Close() error {
	return fh.rwc.Close()
}
