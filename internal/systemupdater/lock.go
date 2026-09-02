package systemupdater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

var ErrUpdateLocked = errors.New("another update operation holds the host lock")

type UpdateLock interface {
	TryLock() (func() error, error)
}

type FileLock struct {
	path string
}

func NewFileLock(stateDir string) *FileLock {
	return &FileLock{path: filepath.Join(stateDir, "update.lock")}
}

func (l *FileLock) TryLock() (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(l.path), 0700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open update lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrUpdateLocked
		}
		return nil, fmt.Errorf("acquire update lock: %w", err)
	}
	return func() error {
		unlockErr := unix.Flock(int(file.Fd()), unix.LOCK_UN)
		closeErr := file.Close()
		if unlockErr != nil {
			return unlockErr
		}
		return closeErr
	}, nil
}
