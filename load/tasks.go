// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"context"
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
