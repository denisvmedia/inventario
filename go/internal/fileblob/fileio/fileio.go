package fileio

import (
	"errors"
	"io"
	"syscall"

	"github.com/spf13/afero"
)

// Move tries os.Rename, falls back to copy+remove on cross-device error.
func Move(fs afero.Fs, src, dst string) error {
	err := fs.Rename(src, dst)
	if err == nil {
		return nil
	}

	// Check if it's a cross-device link error
	if isCrossDeviceError(err) {
		return copyAndRemove(fs, src, dst)
	}
	return err
}

// Platform-independent check for cross-device error.
func isCrossDeviceError(err error) bool {
	// Unix: syscall.EXDEV, Windows: ERROR_NOT_SAME_DEVICE (17)
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EXDEV || errno == 17
	}
	return false
}

// Copies src to dst and removes src.
func copyAndRemove(fs afero.Fs, src, dst string) error {
	in, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := fs.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	if err = out.Sync(); err != nil {
		return err
	}

	if err = fs.Remove(src); err != nil {
		return err
	}

	return nil
}
