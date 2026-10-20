//go:build !windows

package workspace

import (
	"os"
	"syscall"
)

func lockRegistry(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
func unlockRegistry(f *os.File) error       { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
func replaceRegistry(from, to string) error { return os.Rename(from, to) }
func syncRegistryDir(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func openIdentity(path string) (*os.File, error) { return os.Open(path) }
