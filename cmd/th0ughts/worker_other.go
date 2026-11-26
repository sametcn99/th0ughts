//go:build !linux

package main

import "os/exec"

func bindWorkerLifetime(cmd *exec.Cmd) {}
