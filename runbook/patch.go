package runbook

import "fmt"

var patchcmd = `
[ -d .git ] && {
	git am --committer-date-is-author-date $PATCHFILE
	exit $?
}

git init
git config  user.email "robot@boxgo.com"
git config  user.name "robot"
git add -A
git commit -m 'first commit'

git apply $PATCHFILE && {
	git add -A
	git commit -m "apply patch: $(basename $PATCHFILE)"
}
`

// PatchFile help patch
func PatchFile(patch string, r Runtime) error {

	cmd := TaskCmd{name: patchcmd}
	if e := cmd.Run(r, fmt.Sprintf("PATCHFILE=%s\n", patch)); e != nil {
		return fmt.Errorf("Patch %s %s", patch, e)
	}
	return nil
}
