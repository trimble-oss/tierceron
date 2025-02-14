package trcshzigfs

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// trcshzigfs wraps an io.ReadWriteCloser
type trcshzigFileHandle struct {
	rwc io.ReadWriteCloser
}

type TrcshZigReadWriteCloser struct {
	*bytes.Buffer
}

func (rwc *TrcshZigReadWriteCloser) Close() error {
	return nil
}

type trcshZigFile struct {
	fs.Inode
	rwc io.ReadWriteCloser
	tzr *TrcshZigRoot
}

type TrcshZigRoot struct {
	fs.Inode

	zigFiles *map[string]interface{}
	pid      uint32
}

var _ = (fs.NodeOnAdder)((*TrcshZigRoot)(nil))

func (tzr *TrcshZigRoot) OnAdd(ctx context.Context) {
	// OnAdd is called once we are attached to an Inode. We can
	// then construct a tree.  We construct the entire tree, and
	// we don't want parts of the tree to disappear when the
	// kernel is short on memory, so we use persistent inodes.
	for path, trcshZigFileBytes := range *tzr.zigFiles {
		if path == "env" || path == "log" || path == "PluginEventChannelsMap" {
			continue
		}
		dir, base := filepath.Split(path)

		p := &tzr.Inode
		for _, component := range strings.Split(dir, "/") {
			if len(component) == 0 || component == "." {
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

func (tzr *TrcshZigRoot) SetPid(pid uint32) {
	tzr.pid = pid
}

func (tzr *TrcshZigRoot) GetPid() uint32 {
	return tzr.pid
}

func NewTrcshZigFileBytes(zigFileBytes []byte, tzr *TrcshZigRoot) *trcshZigFile {
	// TODO:... *zigFileBytes?
	return &trcshZigFile{rwc: &TrcshZigReadWriteCloser{Buffer: bytes.NewBuffer(zigFileBytes)}, tzr: tzr}
}

func NewTrcshZigFileHandle(rwc io.ReadWriteCloser) *trcshzigFileHandle {
	return &trcshzigFileHandle{rwc: rwc}
}

func (fh *trcshZigFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	caller, ok := fuse.FromContext(ctx)
	if !ok {
		return nil, 0, syscall.EACCES
	}
	pid := caller.Pid
	if pid != fh.tzr.GetPid() {
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

func (fh *trcshzigFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if seeker, ok := fh.rwc.(io.Seeker); ok {
		if _, err := seeker.Seek(off, io.SeekStart); err != nil {
			return nil, syscall.EIO
		}
	} else {
		return nil, syscall.ENOTSUP
	}

	n, err := fh.rwc.Read(dest)
	if err != nil && err != io.EOF {
		return nil, syscall.EIO
	}

	return fuse.ReadResultData(dest[:n]), 0
}

func (fh *trcshzigFileHandle) Write(p []byte) (int, error) {
	return fh.rwc.Write(p)
}

func (fh *trcshzigFileHandle) Close() error {
	return fh.rwc.Close()
}
