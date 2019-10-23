// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"merge/log"
)

type Command struct {
	Cmd     *exec.Cmd
	timeout string

	ctxErr error
	ctx    context.Context
}

// NewCommand new exec.Cmd wrapper Command
func NewCommand(ctx context.Context, name string, args ...string) *Command {

	arg, _ := FromContext(ctx)
	timeout, _ := arg.LookupVar("TIMEOUT")
	timeOut, _ := strconv.Atoi(timeout)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeOut)*time.Second)
	// just need timeout mechanism, but it still can be cancelled bu upper ctx
	_ = cancel

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = arg.Output()
	cmd.Dir = arg.SrcDir(arg.Wd)
	arg.VisitVars(func(k, v string) {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	})
	for k, v := range arg.Vars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return &Command{
		timeout: timeout,
		Cmd:     cmd,
		ctx:     ctx,
	}
}

// CmdRun push child processes into the same process group then run
func (c *Command) Run(stage string) error {

	arg, _ := FromContext(c.ctx)

	//Child processes get the same process group id(PGID) as their parents by default
	c.Cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	waitDone := make(chan struct{})
	defer close(waitDone)
	go func() {
		select {
		case <-c.ctx.Done():
			// kill all processes in the process group by sending a KILL to
			//-PID of the process, which is the same as -PGID. Assuming that
			//the child process did not use setpgid(2) when spawning its
			//own child, this should kill the child along with all of its
			//children on any *Nix systems.
			syscall.Kill(-c.Cmd.Process.Pid, syscall.SIGKILL)
			c.ctxErr = c.ctx.Err()
			log.Warning("Runbook: kill child processes started by %s@%s since %v",
				arg.Owner, stage, c.ctxErr)
		case <-waitDone:
		}
	}()

	if err := c.Cmd.Start(); err != nil {
		return err
	}

	err := c.Cmd.Wait()
	if c.ctxErr != nil {
		switch c.ctxErr {
		case context.DeadlineExceeded:
			return fmt.Errorf("Runbook expire on %s@%s over %s seconds",
				arg.Owner, stage, c.timeout)
		default:
			return fmt.Errorf("Runbook failed on %s@%s since %s",
				arg.Owner, stage, c.ctxErr)
		}
	}

	if err != nil {
		return fmt.Errorf("Runbook failed on %s@%s since %s",
			arg.Owner, stage, err)
	}
	return nil
}