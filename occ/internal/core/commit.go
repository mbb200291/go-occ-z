package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
)

const backupSuffix = ".jsonl"

var ErrCommitUncertain = errors.New("occ: commit durability uncertain")

type BackupFileHandler struct {
	txnId string
}

type BackupEntry struct {
	Exist   bool
	Content []byte
	Target  string
}

func NewBackupFileHandler(txnId string) *BackupFileHandler {
	return &BackupFileHandler{
		txnId: txnId,
	}
}

func (bfh *BackupFileHandler) path() string {
	return filepath.Join(".occ", bfh.txnId+backupSuffix)
}

// SaveBackup appends one record. Call Sync before changing its target.
// A handler is used by one transaction at a time.
func (bfh *BackupFileHandler) SaveBackup(target string, exist bool, content []byte) error {
	if err := os.MkdirAll(".occ", 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(bfh.path(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(BackupEntry{Target: target, Exist: exist, Content: content})
	return errors.Join(err, file.Close())
}

func syncBackupPath(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	err = file.Sync()
	return errors.Join(err, file.Close())
}

func (bfh *BackupFileHandler) Sync() error {
	// Persist both file contents and the directory entries that locate them.
	for _, path := range []string{bfh.path(), ".occ", "."} {
		if err := syncBackupPath(path); err != nil {
			return fmt.Errorf("sync %q: %w", path, err)
		}
	}
	return nil
}

func (bfh *BackupFileHandler) RemoveBackups() error {
	if err := os.Remove(bfh.path()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := syncBackupPath(".occ"); err != nil {
		return fmt.Errorf("%w: sync backup deletion: %w", ErrCommitUncertain, err)
	}
	return nil
}

func (bfh *BackupFileHandler) LoadBackups() iter.Seq2[*BackupEntry, error] {
	return func(yield func(*BackupEntry, error) bool) {
		file, err := os.Open(bfh.path())
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			yield(nil, err)
			return
		}
		defer file.Close()
		reader := bufio.NewReader(file)
		for {
			line, err := reader.ReadBytes('\n')
			// An unfinished last record was never followed by a successful Sync.
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield(nil, err)
				return
			}
			var entry BackupEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				yield(nil, fmt.Errorf("decode backup entry: %w", err))
				return
			}
			if !yield(&entry, nil) {
				return
			}
		}
	}
}

// Commit writes the transaction and returns failures without rolling back.
// The caller controls rollback and handles rollback failures.
func Commit(txn Transaction, adpt TargetAdapter) error {
	bfh := NewBackupFileHandler(txn.GetID())

	for target, newExists := range txn.GetWriteSet().All() {
		// backup target
		ori, oldExists, err := adpt.Read(target)
		if err != nil {
			return err
		}
		if !oldExists && newExists { // CREAT
			if err := bfh.SaveBackup(target, false, nil); err != nil {
				return err
			}
		} else if oldExists { // UPDATE or DELETE
			oriB, err := adpt.Serialize(ori)
			if err != nil {
				return err
			}
			if err := bfh.SaveBackup(target, true, oriB); err != nil {
				return err
			}
		} else {
			continue // The target is already absent.
		}

		if err = bfh.Sync(); err != nil {
			err = fmt.Errorf("sync transaction backup: %w", err) // TODO
			return err
		}

		// make change
		if newExists {
			if err := adpt.Write(target, txn.GetData(target)); err != nil {
				return err
			}
		} else {
			if err := adpt.Delete(target); err != nil {
				return err // if target not exist in source, skip it
			}
		}
	}

	// Removing the backup is the commit point.
	return bfh.RemoveBackups()
}

// Withdraw restores targets from backup entries.
func Withdraw(txnId string, adpt TargetAdapter) error {
	bfh := NewBackupFileHandler(txnId)
	for backEntry, err := range bfh.LoadBackups() {
		if err != nil {
			return err
		}
		target := backEntry.Target
		if !backEntry.Exist {
			if err := adpt.Delete(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to revert created target %q: %w", target, err)
			}
			continue
		}
		ori, err := adpt.UnSerialize(backEntry.Content)
		if err != nil {
			return fmt.Errorf("failed to unserialize backup for %q: %w", target, err)
		}
		if err := adpt.Write(target, ori); err != nil {
			return fmt.Errorf("failed to revert target %q: %w", target, err)
		}
	}
	return bfh.RemoveBackups()
}

// TargetAdapter reads and changes targets. Read reports a missing target as
// (nil, false, nil). Write must create or replace. Delete must tolerate a
// missing target, including during recovery.
// For power-loss recovery, successful Write and Delete calls must persist
// their changes, including any directory entries they create or remove.
type TargetAdapter interface {
	Read(string) (any, bool, error)
	Write(string, any) error
	Delete(string) error
	Serialize(any) ([]byte, error)
	UnSerialize([]byte) (any, error)
}

// Recovery restores transactions whose backup files are still present.
func Recovery(adpt TargetAdapter) error {
	entries, err := os.ReadDir(".occ")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		if entry.Name() == ".occ.lock" {
			continue
		}
		txnId, ok := strings.CutSuffix(entry.Name(), backupSuffix)
		if !ok || txnId == "" || !entry.Type().IsRegular() {
			errs = append(errs, fmt.Errorf("unsupported entry in .occ: %q", entry.Name()))
			continue
		}
		if err := Withdraw(txnId, adpt); err != nil {
			errs = append(errs, fmt.Errorf("recover transaction %s: %w", txnId, err))
		}
	}
	return errors.Join(errs...)
}
