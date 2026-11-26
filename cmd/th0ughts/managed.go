package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/updates"
)

const restartExit = 75

var restartRequested = errors.New("restart to apply update")

func supervise(ctx context.Context, dir string, args []string) error {
	if !updates.Supported() {
		return updates.ErrUnsupported
	}
	vault, err := feed.NewVault(os.Getenv("TH0UGHTS_MASTER_KEY"))
	if err != nil {
		return err
	}
	manager := updates.New(dir, vault)
	lock, err := manager.LockLauncher()
	if err != nil {
		return err
	}
	defer lock.Unlock()
	self, err := os.Executable()
	if err != nil {
		return err
	}
	for ctx.Err() == nil {
		manager.Prune()
		boots, err := manager.Boots(self)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ignoring invalid pending update:", err)
		}
		var last error
		var restart bool
		for _, boot := range boots {
			var ready bool
			ready, last = runWorker(ctx, manager, boot, args)
			if ctx.Err() != nil {
				return nil
			}
			var exit *exec.ExitError
			if ready && errors.As(last, &exit) && exit.ExitCode() == restartExit {
				restart = true
				break
			}
			if ready {
				return last
			}
			fmt.Fprintln(os.Stderr, "Could not start", boot.Version, last)
		}
		if !restart {
			return last
		}
	}
	return nil
}

// The worker signals readiness after opening the store and binding the listener.
// Only a ready candidate becomes the executable selected on subsequent starts.
func runWorker(ctx context.Context, manager *updates.Manager, boot updates.Boot, args []string) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r, w, err := os.Pipe()
	if err != nil {
		return false, err
	}
	defer r.Close()
	cmd := exec.CommandContext(ctx, boot.Path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.ExtraFiles = []*os.File{w}
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 15 * time.Second
	bindWorkerLifetime(cmd)
	err = cmd.Start()
	w.Close()
	if err != nil {
		return false, err
	}
	started := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(r)
		if s.Scan() {
			started <- s.Text()
		} else {
			started <- ""
		}
	}()
	var timeout <-chan time.Time
	if boot.Trial {
		timer := time.NewTimer(2 * time.Minute)
		defer timer.Stop()
		timeout = timer.C
	}
	select {
	case version := <-started:
		if version == boot.Version {
			if err = manager.Confirm(boot); err == nil {
				return true, cmd.Wait()
			}
		} else {
			err = errors.New("worker did not announce the expected version")
		}
	case <-timeout:
		err = errors.New("updated worker startup timed out")
	case <-ctx.Done():
		err = ctx.Err()
	}
	cancel()
	return false, errors.Join(err, cmd.Wait())
}
