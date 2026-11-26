package main

import (
	"os/exec"
	"syscall"
)

func bindWorkerLifetime(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
