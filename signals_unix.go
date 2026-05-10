//go:build !windows

package main

import (
	"os"
	"syscall"
)

func suspend(proc *os.Process) { proc.Signal(syscall.SIGSTOP) }
func cont(proc *os.Process)    { proc.Signal(syscall.SIGCONT) }
