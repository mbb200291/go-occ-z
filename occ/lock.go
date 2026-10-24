package occ

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
)

var ErrDirectoryInUse = errors.New("occ: state directory is already in use")

// Keep successful locks reachable until process exit, even if a Manager is
// garbage-collected. This mutex is used only during initialization.
var processLocks struct {
	sync.Mutex
	locks []*flock.Flock
}

func retainLock(lock *flock.Flock) {
	processLocks.Lock()
	defer processLocks.Unlock()
	processLocks.locks = append(processLocks.locks, lock)
}

func acquireLock() (*flock.Flock, error) {
	root, err := filepath.Abs(".occ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(root, ".occ.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, errors.Join(err, lock.Close())
	}
	if !ok {
		return nil, errors.Join(ErrDirectoryInUse, lock.Close())
	}
	return lock, nil
}
