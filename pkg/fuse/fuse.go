package fuse

import (
	"context"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"git.backbone/corpix/gpgfs/pkg/errors"
	"git.backbone/corpix/gpgfs/pkg/log"
)

type (
	Fuse struct {
		fs.Inode

		log    log.Logger
		key    *Enclave
		config Config
		source string
		target string
	}
	Server     = fuse.Server
	ReadResult = fuse.ReadResult
	Status     = fuse.Status
	Node       interface{ fs.NodeOnAdder }
)

var _ = (Node)((*Fuse)(nil))

//

func (f *Fuse) warn(path string, d iofs.DirEntry, err error) *log.Event {
	e := f.log.
		Warn().
		Str("path", path)
	if err != nil {
		e = e.Errs("error", []error{err})
	}
	return e
}

func (f *Fuse) OnAdd(ctx context.Context) {
	// does nothing, whole secrets store is preloaded in f.Preload
	// because this func can not return errors
}

func (f *Fuse) Preload(ctx context.Context) error {
	keyBuf, err := f.key.Open()
	if err != nil {
		panic(errors.Wrap(err, "failed to obtain locked buffer from enclave"))
	}
	defer keyBuf.Destroy()

	return filepath.WalkDir(
		f.source,
		func(path string, d iofs.DirEntry, err error) error {
			if err != nil {
				f.
					warn(path, d, err).
					Msg("skipping file because of error")
				return nil
			}

			var (
				inode       *Inode
				inodePath   = path
				inodeParent = &f.Inode
			)

			//

			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				f.
					warn(path, d, err).
					Msg("skipping unsupported file type")
				return nil
			}
			if !strings.HasSuffix(path, EncryptedSuffix) {
				f.
					warn(path, d, err).
					Msgf("skipping file without required suffix %q", EncryptedSuffix)
				return nil
			}

			//

			encBuf, err := os.ReadFile(path)
			if err != nil {
				f.
					warn(path, d, err).
					Msg("skipping file because of error")
				return nil
			}

			plainMessage, err := Decrypt(keyBuf, encBuf)
			if err != nil {
				f.
					warn(path, d, err).
					Msg("skipping file because of error")
				return nil
			}

			//

			inodePath = strings.TrimSuffix(inodePath, EncryptedSuffix)
			inodePath, err = filepath.Rel(f.source, inodePath)
			if err != nil {
				f.
					warn(path, d, err).
					Msg("skipping file because of error")
				return nil
			}

			dir, base := filepath.Split(filepath.Clean(inodePath))
			for _, component := range strings.Split(dir, string(filepath.Separator)) {
				if len(component) == 0 {
					continue
				}

				inode = inodeParent.GetChild(component)
				if inode == nil {
					inode = inodeParent.NewPersistentInode(
						ctx,
						&Inode{},
						FSAttr{Mode: fuse.S_IFDIR},
					)
					f.log.
						Info().
						Str("inode", inode.String()).
						Str("dir", dir).
						Str("base", base).
						Str("path", path).
						Str("inode-path", inodePath).
						Str("component", component).
						Msg("mounting directory component")
					inodeParent.AddChild(component, inode, true)
				}
				inodeParent = inode
			}

			//

			attr := Attr{FuseAttr: &FuseAttr{Mode: 0400}}
			attrPath := strings.TrimSuffix(path, EncryptedSuffix) + AttrSuffix
			_, err = os.Stat(attrPath)
			if err == nil {
				attrBuf, err := os.ReadFile(attrPath)
				if err != nil {
					return errors.Wrapf(
						err, "failed to load attrs from %q",
						attrPath,
					)
				}
				err = yaml.Unmarshal(attrBuf, &attr)
				if err != nil {
					return errors.Wrapf(
						err, "failed to parse attrs from %q",
						attrPath,
					)
				}

				err = attr.Expand()
				if err != nil {
					return errors.Wrap(err, "faield to expand fuse node attributes")
				}

				//

				f.log.
					Debug().
					Str("dir", dir).
					Str("base", base).
					Str("path", path).
					Str("inode-path", inodePath).
					Str("attr-path", attrPath).
					Interface("attr", attr).
					Msg("loaded attr")
			} else {
				if !os.IsNotExist(err) {
					return err
				}
			}

			//

			inode = inodeParent.NewPersistentInode(
				ctx,
				NewFile(
					f.log,
					attr,
					NewEnclave(plainMessage.Data),
				),
				FSAttr{},
			)

			f.log.
				Info().
				Str("inode", inode.String()).
				Str("dir", dir).
				Str("base", base).
				Str("path", path).
				Str("inode-path", inodePath).
				Msg("mounting file")
			inodeParent.AddChild(base, inode, false)

			return nil
		},
	)
}

func (f *Fuse) Mount() (*Server, error) {
	opts := &fs.Options{}
	opts.Debug = f.config.Debug

	server, err := fs.Mount(f.target, f, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to mount %q", f.target)
	}

	return server, nil
}

func New(c Config, l log.Logger, key *Enclave, source string, target string) (*Fuse, error) {
	_, err := os.Stat(source)
	if err != nil {
		return nil, errors.Wrap(err, "error while stat source")
	}

	_, err = os.Stat(target)
	if err != nil {
		return nil, errors.Wrap(err, "error while stat target")
	}

	//

	absSource, err := filepath.Abs(source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get absolute path of source")
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get absolute path of target")
	}

	return &Fuse{
		config: c,
		log:    l,
		key:    key,
		source: absSource,
		target: absTarget,
	}, nil
}
