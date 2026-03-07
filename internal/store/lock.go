package store

import (
	"fmt"
	"os"
)

type Lock struct {
	file *os.File
	path string
}

func AcquireLock(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("database lock already held: %s", path)
		}
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	return &Lock{file: file, path: path}, nil
}

func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	if err := l.file.Close(); err != nil {
		return err
	}
	return os.Remove(l.path)
}
