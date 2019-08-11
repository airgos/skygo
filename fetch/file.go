package fetch

import (
	"boxgo/fetch/utils"
	"os"
	"path/filepath"
)

type locate struct{}

// FILE file scheme fetcher
var FILE locate

// TODO: print log(file, filePath) if file is not found
func (locate) Fetch(url, dest string, f Fetcher) error {

	// skip file://
	url = url[7:]
	for _, d := range f.FilePath() {

		path := filepath.Join(d, url)
		fileinfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		target := filepath.Join(f.WorkPath(), filepath.Base(url))
		utils.CopyFile(target, fileinfo.Mode(), file)
		// TODO: copy when mod time and content is chagned
		break
	}
	return nil
}
