package main

import (
	"runtime"

	"git.backbone/corpix/gpgfs/cli"
)

func init() { runtime.GOMAXPROCS(runtime.NumCPU()) }
func main() { cli.Run() }
