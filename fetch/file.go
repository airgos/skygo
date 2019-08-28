package fetch

import (
	"context"
	"merge/fetch/utils"
	"os"
	"path/filepath"
)

func file(ctx context.Context, filePath []string, wd string, url string) error {

	// skip file://
	url = url[7:]
	for _, d := range filePath {

		path := filepath.Join(d, url)
		fileinfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		target := filepath.Join(wd, filepath.Base(url))
		utils.CopyFile(target, fileinfo.Mode(), file)
		// TODO: copy when mod time and content is chagned
		break
	}
	return nil
}
