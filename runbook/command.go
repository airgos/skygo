// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"skygo/utils/log"
)

// Command represents command of runbook
type Command struct {
	Cmd *exec.Cmd
}

// NewCommand new exec.Cmd wrapper Command
func NewCommand(ctx Context, name string, args ...string) *Command {

	cmd := exec.CommandContext(ctx.Ctx(), name, args...)
	cmd.Stdout, cmd.Stderr = ctx.Output()

	cmd.Env = os.Environ() // inherits OS global env, like HTTP_PROXY
	ctx.Range(func(key, value string) {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	})
	return &Command{
		Cmd: cmd,
	}
}

// CmdRun push child processes into the same process group then run
func (c *Command) Run(ctx Context, stage string) error {

	//Child processes get the same process group id(PGID) as their parents by default
	c.Cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	waitDone := make(chan struct{})
	defer close(waitDone)
	go func() {
		select {
		case <-ctx.Ctx().Done():
			select {
			case <-waitDone: // command had been finished, ignore cancelation
			default:
				// kill all processes in the process group by sending a KILL to
				//-PID of the process, which is the same as -PGID. Assuming that
				//the child process did not use setpgid(2) when spawning its
				//own child, this should kill the child along with all of its
				//children on any *Nix systems.
				syscall.Kill(-c.Cmd.Process.Pid, syscall.SIGKILL)
				err := ctx.Ctx().Err()
				log.Trace("Runbook: kill child processes started by %s@%s since %v",
					ctx.Owner(), stage, err)
			}
		case <-waitDone:
		}
	}()

	if err := c.Cmd.Start(); err != nil {
		return err
	}

	if err := c.Cmd.Wait(); err != nil {
		return fmt.Errorf("Runbook failed on %s@%s since %s",
			ctx.Owner(), stage, err)
	}
	return nil
}
