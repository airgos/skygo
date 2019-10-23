// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
)

func tmuxPanes(ctx context.Context) []string {
	var outPanes []string
	var buf bytes.Buffer

	if _, err := exec.LookPath("tmux"); err != nil {
		return outPanes
	}

	if _, err := exec.LookPath("tty"); err != nil {
		return outPanes
	}

	// list panes in current session
	cmd := exec.CommandContext(ctx, "tmux", "list-panes", "-sF", "#{pane_tty}")
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return outPanes
	}
	panes := buf.String()

	cmd = exec.CommandContext(ctx, "tty")
	buf.Reset()
	cmd.Stdin = os.Stdin // inherits os.Stdin for utility tty
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return outPanes
	}
	tty := buf.String()

	if !strings.Contains(panes, tty) {
		return outPanes
	}

	return strings.Fields(strings.Replace(panes, tty, "", 1))
}
