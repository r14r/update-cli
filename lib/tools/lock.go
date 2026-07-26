package tools

import (
	"errors"
	"fmt"
	"os"
)

type Lock struct {
	path string
}

func AcquireLock(path string) (*Lock, error) {
	if err := os.Mkdir(path, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("ein anderes Update läuft bereits: %s", path)
		}
		return nil, fmt.Errorf("Update-Sperre kann nicht erstellt werden: %w", err)
	}
	return &Lock{path: path}, nil
}

func (lock *Lock) Release() {
	if lock != nil && lock.path != "" {
		_ = os.Remove(lock.path)
	}
}
