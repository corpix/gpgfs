# gpgfs

An implementation of the FUSE for GPG encrypted filesystem trees.

This service could be used with [unix password manager](https://www.passwordstore.org/).

Example `source` filesystem tree:

```console
$ tree test/secrets/
test/secrets/
├── msg
├── msg-rsa.gpg
└── subdir
    ├── msg2
    ├── msg2-rsa.gpg
    └── msg2-rsa.yml
```

Mount `target` filesystem into `~/tmp/fuse/mountpoint`:

> superuser privileges is not required

```console
$ mkdir -p ~/tmp/fuse/mountpoint
$ go run ./main.go mount --source ./test/secrets/ --target ~/tmp/fuse/mountpoint
```

> take a look at [config.yml](config.yml)

At this point you should be able to view decrypted contents of any `.gpg` file from `source` directory:

```console
$ tree ~/tmp/fuse/mountpoint
/home/user/tmp/fuse/mountpoint
├── msg-rsa
└── subdir
    └── msg2-rsa

$ ls -la ~/tmp/fuse/mountpoint/
.r-------- 13 root  1 Jan  1970 msg-rsa
drwxr-xr-x  - root  1 Jan  1970 subdir

$ ls -la ~/tmp/fuse/mountpoint/subdir/
.rw------- 13 root  1 Jan  1970 msg2-rsa

$ cat ~/tmp/fuse/mountpoint/msg-rsa
test message
```

Additional attributes could be set for any file in the `target` mountpoint which has a corresponding `source` file with `.gpg` extension.

To set attributes create `.yml` file with the same name as `.gpg` file has:

```console
$ ls -la test/secrets/subdir/msg2-rsa.*
.rw------- 463 user  8 Sep 12:32 test/secrets/subdir/msg2-rsa.gpg
.rw-r--r--  11 user  9 Sep 01:24 test/secrets/subdir/msg2-rsa.yml

$ cat test/secrets/subdir/msg2-rsa.yml
mode: 0600
```

Attributes available:

```yml
Atime:      0
Atimensec:  0
Blksize:    0
Blocks:     0
Ctime:      0
Ctimensec:  0
Ino:        0
Mode:       384
Mtime:      0
Mtimensec:  0
Nlink:      0
Padding:    0
Rdev:       0
Size:       0
Uid:        0
Gid:        0
```

## development

- make sure you have `git`, `make`, `go`, `nix`
- clone this repository
- `cd` into
- then you could want to run one of the following commands

```console
$ make build
$ make run
$ make test
$ make help
...
```
