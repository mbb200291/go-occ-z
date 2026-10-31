package core

import (
	"bufio"
	"bytes"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Commit writes the transaction and returns failures without rolling back.
// The caller controls rollback and handles rollback failures.
func Commit(txn Transaction, adpt TargetAdapter) error {
	for target := range txn.GetWriteSet().All() {
		// backup target
		ori, err := adpt.Read(target) // TODO: handle new file create and remove
		if err != nil {
			return err
		}
		oriB, err := adpt.Serialize(ori)
		if err != nil {
			return err
		}
		if err := saveBackup(txn.GetID(), target, oriB); err != nil {
			return err
		}

		// write log first, even target not been write -> idempotent recover by backup
		if err := appendLog(txn.GetID(), "WRITE", target); err != nil {
			return err
		}

		// write target
		if err := adpt.Write(target, txn.GetWriteData(target)); err != nil {
			return err
		}
	}

	// write COMMIT log
	if err := appendLog(txn.GetID(), "COMMIT", ""); err != nil {
		return err

	}

	// remove txn backup folder
	go func() {
		if err := cleanup(txn.GetID()); err != nil {
			log.Printf("failed to clean txn backups %s: %v", txn.GetID(), err)
		}
	}()

	return nil
}

// Withdraw restores uncommitted targets and reports whether a complete COMMIT was found.
func Withdraw(txnId string, adpt TargetAdapter) (committed bool, err error) {
	// load log files, determine target to revert
	revertTargets, committed, err := loadTargetsToRevert(txnId)
	if err != nil {
		return false, err
	}

	if committed {
		log.Printf(
			"txn %s commited, but maybe fail to remove log", txnId,
		)
	} else {
		for target := range revertTargets {
			// Load backup
			orib, err := loadBackup(txnId, target)
			if err != nil {
				return committed, fmt.Errorf(
					"failed to load backup for %q: %w",
					target, err,
				)
			}

			ori, err := adpt.UnSerialize(orib)
			if err != nil {
				return committed, fmt.Errorf(
					"failed to unserialize backup for %q: %w",
					target, err,
				)
			}

			// revert target
			if err := adpt.Write(target, ori); err != nil {
				return committed, fmt.Errorf(
					"failed to revert target %q: %w",
					target, err,
				)
			}
			if err := appendLog(txnId, "REVERT", target); err != nil {
				return committed, err
			}
			if err := discardBackup(txnId, target); err != nil {
				return committed, err
			}
		}
	}

	// remove txn data
	if err := cleanup(txnId); err != nil {
		log.Printf("failed to remove txn data %s: %v", txnId, err)
		return committed, err // keep remaining state for another recovery
	}

	return committed, nil
}

type TargetAdapter interface {
	Read(string) (any, error)
	Write(string, any) error
	Serialize(any) ([]byte, error)
	UnSerialize([]byte) (any, error)
}

// Read unfinished txn in txn logs and carry on undone txn rollback works
// Return txnID with success "COMMIT" log
func Recovery(adpt TargetAdapter) ([]string, error) {
	entries, err := os.ReadDir(".occ")
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var successTxnIds []string
	var errs []error
	for _, entry := range entries {
		if entry.Name() == ".occ.lock" {
			continue
		}
		txnId := entry.Name()
		if !entry.IsDir() {
			errs = append(errs, fmt.Errorf("invalid transaction directory %q", entry.Name()))
			continue
		}
		committed, err := Withdraw(txnId, adpt)
		if committed {
			successTxnIds = append(successTxnIds, txnId)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("recover transaction %s: %w", txnId, err))
		}
	}
	return successTxnIds, errors.Join(errs...)
}

func transactionDir(txnId string) string { return filepath.Join(".occ", txnId) }
func journalPath(txnId string) string    { return filepath.Join(transactionDir(txnId), "journal") }
func backupPath(txnId, target string) string {
	return filepath.Join(transactionDir(txnId), "backup", encodeTarget(target))
}

func discardBackup(txnId, target string) error {
	if err := os.Remove(backupPath(txnId, target)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var pathEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func encodeTarget(target string) string {
	return "x_" + pathEncoding.EncodeToString([]byte(target))
}

func decodeTarget(encoded string) (string, error) {
	if !strings.HasPrefix(encoded, "x_") {
		return "", fmt.Errorf("invalid encoded target: %q", encoded)
	}

	data, err := pathEncoding.DecodeString(encoded[2:])
	return string(data), err
}

func saveBackup(txnId, target string, data []byte) error {
	path := backupPath(txnId, target)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}

	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}

	return file.Close()
}

func loadBackup(txnId, target string) ([]byte, error) {
	path := backupPath(txnId, target)
	return os.ReadFile(path)
}

func loadTargetsToRevert(txnId string) (map[string]bool, bool, error) {
	path := journalPath(txnId)
	revertTargets := make(map[string]bool)

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return revertTargets, false, nil
		}
		return nil, false, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, false, err
	}
	// Parse only newline-terminated records. Validate them before changing disk.
	end := bytes.LastIndexByte(data, '\n') + 1
	committed := false
	reader := bufio.NewScanner(bytes.NewReader(data[:end]))
	reader.Buffer(make([]byte, 4096), len(data)+1)
	for reader.Scan() {
		line := reader.Text()
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if committed {
			return nil, false, fmt.Errorf(
				"unexpected log after COMMIT: %q", line,
			)
		}

		if strings.TrimSpace(line) == "COMMIT" {
			committed = true
			continue
		}

		action, targetR, ok := strings.Cut(line, " ")
		if !ok || targetR == "" {
			return nil, false, fmt.Errorf(
				"invalid log entry: %q", line,
			)
		}
		target, err := decodeTarget(targetR)
		if err != nil {
			return nil, false, fmt.Errorf(
				"invalid log entry: %q", line,
			)
		}
		switch action {
		case "WRITE":
			revertTargets[target] = true
		case "REVERT":
			delete(revertTargets, target)
		default:
			return nil, false, fmt.Errorf(
				"unknown log action: %q", action,
			)
		}
	}

	if err := reader.Err(); err != nil {
		return nil, false, err
	}
	if end < len(data) {
		if err := truncateLogTail(file, int64(end)); err != nil {
			return nil, false, err
		}
	}

	return revertTargets, committed, nil
}

// Recovery requires exclusive access to this transaction's log. Sync the
// truncation before restoring data or appending further recovery records.
func truncateLogTail(file *os.File, size int64) error {
	if err := file.Truncate(size); err != nil {
		return fmt.Errorf("truncate incomplete transaction log: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync truncated transaction log: %w", err)
	}
	return nil
}

func appendLog(txnId, action, target string) error {
	if err := os.MkdirAll(transactionDir(txnId), 0755); err != nil {
		return err
	}

	path := journalPath(txnId)

	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	line := action + "\n"
	if action != "COMMIT" {
		line = action + " " + encodeTarget(target) + "\n"
	}
	if _, err := file.WriteString(line); err != nil {
		return err
	}

	return file.Sync()
}

// Retain the journal until backups are removed
func cleanup(txnId string) error {
	if err := os.RemoveAll(filepath.Join(transactionDir(txnId), "backup")); err != nil {
		return err
	}
	if err := os.Remove(journalPath(txnId)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(transactionDir(txnId)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
