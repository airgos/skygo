// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"merge/log"
)

type Command struct {
	Cmd *exec.Cmd

	ctxErr error
	ctx    context.Context
}

// NewCommand new exec.Cmd wrapper Command
func NewCommand(ctx context.Context, name string, args ...string) *Command {

	arg, _ := FromContext(ctx)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = arg.Output()
	if dir, ok := arg.LookupVar("S"); ok {
		cmd.Dir = dir
	}

	cmd.Env = os.Environ() // inherits OS global env, like HTTP_PROXY
	arg.visitVars(func(key, value string) {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	})
	return &Command{
		Cmd: cmd,
		ctx: ctx,
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
			select {
			case <-waitDone: // command had been finished, ignore cancelation
			default:
				// kill all processes in the process group by sending a KILL to
				//-PID of the process, which is the same as -PGID. Assuming that
				//the child process did not use setpgid(2) when spawning its
				//own child, this should kill the child along with all of its
				//children on any *Nix systems.
				syscall.Kill(-c.Cmd.Process.Pid, syscall.SIGKILL)
				c.ctxErr = c.ctx.Err()
				log.Trace("Runbook: kill child processes started by %s@%s since %v",
					arg.Owner, stage, c.ctxErr)
			}
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
			timeout, _ := arg.LookupVar("TIMEOUT")
			return fmt.Errorf("Runbook expire on %s@%s over %s seconds",
				arg.Owner, stage, timeout)
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
