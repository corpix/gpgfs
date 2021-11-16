package fuse

import (
	"context"
	"os/user"
	"strconv"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"git.backbone/corpix/gpgfs/pkg/errors"
	"git.backbone/corpix/gpgfs/pkg/log"
)

const (
	EncryptedSuffix = ".gpg"
	AttrSuffix      = ".yml"
)

type (
	Attr struct {
		*FuseAttr

		User  string
		Group string
	}
	FuseAttr = fuse.Attr
	FSAttr   = fs.StableAttr
	Inode    = fs.Inode

	File struct {
		Inode

		mu      sync.Mutex
		log     log.Logger
		attr    Attr
		content *Enclave
	}
	FileNode interface {
		fs.NodeOpener
		fs.NodeReader
		fs.NodeWriter
		fs.NodeSetattrer
		fs.NodeGetattrer
		fs.NodeFlusher
	}
)

var _ = (FileNode)((*File)(nil))

//

func (a *Attr) Expand() error {
	if a.User != "" {
		u, err := user.Lookup(a.User)
		if err != nil {
			return err
		}

		id, err := strconv.ParseUint(u.Uid, 10, 32)
		if err != nil {
			return errors.Wrapf(err, "failed to parseint given uid %q", u.Uid)
		}

		a.FuseAttr.Uid = uint32(id)
	}
	if a.Group != "" {
		g, err := user.LookupGroup(a.Group)
		if err != nil {
			return err
		}

		id, err := strconv.ParseUint(g.Gid, 10, 32)
		if err != nil {
			return errors.Wrapf(err, "failed to parseint given gid %q", g.Gid)
		}

		a.FuseAttr.Gid = uint32(id)
	}

	return nil
}

//

func (f *File) errno(msg string, err error, errno syscall.Errno) syscall.Errno {
	f.log.
		Error().
		Interface("errno", errno).
		Err(err).
		Msg(msg)
	return errno
}

func (f *File) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	return nil, fuse.FOPEN_KEEP_CACHE, fs.OK
}

func (f *File) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	return 0, syscall.ENOSYS
}

func (f *File) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()

	buf, err := f.content.Open()
	if err != nil {
		return nil, f.errno(
			"got an error while opening file content enclave",
			err, syscall.EIO,
		)
	}
	// NOTE: buf resources freed by readResult.Done()
	// so, consider all following code as "critical section" :)
	// defer buf.Destroy()

	end := off + int64(len(dest))
	if end > int64(buf.Size()) {
		end = int64(buf.Size())
	}

	return NewReadResult(buf, off, end), fs.OK
}

func (f *File) Flush(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	return 0
}

func (f *File) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	return syscall.ENOSYS
}

func (f *File) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()

	buf, err := f.content.Open()
	if err != nil {
		return f.errno(
			"got an error while opening file content enclave",
			err, syscall.EIO,
		)
	}
	defer buf.Destroy()

	out.Attr = *f.attr.FuseAttr
	out.Attr.Size = uint64(len(buf.Bytes()))

	return fs.OK
}

func NewFile(l log.Logger, attr Attr, content *Enclave) *File {
	return &File{
		log:     l,
		attr:    attr,
		content: content,
	}
}

//

type readResult struct {
	*LockedBuffer

	offset int64
	end    int64
}

func (r *readResult) Done() {
	r.LockedBuffer.Destroy()
}

func (r *readResult) Bytes(_ []byte) ([]byte, Status) {
	return r.LockedBuffer.Bytes()[r.offset:r.end], fuse.OK
}

func NewReadResult(lb *LockedBuffer, offset int64, end int64) ReadResult {
	return &readResult{
		LockedBuffer: lb,
		offset:       offset,
		end:          end,
	}
}
