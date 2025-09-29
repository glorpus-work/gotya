package fsutil

import (
	"io"
	"os"
)

// Copy copies the contents of srcFile to dstFile.
func Copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer func() { _ = out.Close() }()

	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}

	defer func() { _ = in.Close() }()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

// CreateFilePerm creates a new file with the specified permissions.
func CreateFilePerm(name string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
}
