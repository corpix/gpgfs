package main

import (
	"context"
	"flag"
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type Fuse struct {
	fs.Inode
}

var _ = (fs.NodeOpener)((*Fuse)(nil))
var _ = (fs.NodeGetattrer)((*Fuse)(nil))
var _ = (fs.NodeOnAdder)((*Fuse)(nil))

func (f *Fuse) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	return nil, fuse.FOPEN_KEEP_CACHE, fs.OK
}

func (r *Fuse) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0400
	return 0
}

func (r *Fuse) OnAdd(ctx context.Context) {
	ch := r.NewPersistentInode(
		ctx, &fs.MemRegularFile{
			Data: []byte("oh, hi"),
			Attr: fuse.Attr{
				Mode: 0400,
			},
		}, fs.StableAttr{Ino: 2})
	r.AddChild("file.txt", ch, false)
}

func main() {
	debug := flag.Bool("debug", false, "print debug data")
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  hello MOUNTPOINT")
	}
	opts := &fs.Options{}
	opts.Debug = *debug
	server, err := fs.Mount(flag.Arg(0), &Fuse{}, opts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	server.Wait()
}
