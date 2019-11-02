// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"context"
	"fmt"
	"os"

	"merge/log"
	"merge/runbook"
)

func cleanall(ctx context.Context) error {

	arg, _ := runbook.FromContext(ctx)
	wd := arg.GetVar("WORKDIR")

	os.RemoveAll(wd)
	log.Trace("Remove working dir %s", wd)
	return nil
}

func printenv(ctx context.Context) error {

	arg, _ := runbook.FromContext(ctx)
	fmt.Println()
	arg.Range(func(k, v string) {
		fmt.Printf("%12s:\t%s\n", k, v)
	})
	return nil
}
