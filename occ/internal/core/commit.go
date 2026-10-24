package core

import (
	"bufio"
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
	path := filepath.Join(".occ", encodeTarget(txnId))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	for target := range txn.GetWriteSet().All() {
		// backup target
		ori, err := adpt.Read(target)
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
		if err := adpt.Write(target, txn.GetOutcome(target)); err != nil {
			return err
		}
	}

	// write COMMIT log
	if err := appendLog(txn.GetID(), "COMMIT", ""); err != nil {
		return err

	}

	// remove txn backup folder
	if err := cleanup(txn.GetID()); err != nil {
		log.Printf("failed to clean txn backups %s: %v", txn.GetID(), err)
		return nil // keep log file in order to be able to redo cleanup backups
	}

	return nil
}

// revert target to backup when txn not yet complete write target and remove intermediate products
func Withdraw(id string, adpt TargetAdapter) error {
	// load log files, determine target to revert
	toRevert, committed, err := loadTargetsToRevert(id)
	if err != nil {
		return err
	}

	if committed {
		log.Printf(
			"txn %s commited, but maybe fail to remove log", id,
		)
	} else {
		for target := range toRevert {
			// Load backup
			orib, err := loadBackup(id, target)
			if err != nil {
				return fmt.Errorf(
					"failed to load backup for %q: %w",
					target, err,
				)
			}

			ori, err := adpt.UnSerialize(orib)
			if err != nil {
				return fmt.Errorf(
					"failed to unserialize backup for %q: %w",
					target, err,
				)
			}

			// revert target
			if err := adpt.Write(target, ori); err != nil {
				return fmt.Errorf(
					"failed to revert target %q: %w",
					target, err,
				)
			}

		}
	}

	// remove txn backups
	if err := cleanup(id); err != nil {
		log.Printf("failed to remove txn backups %s: %v", id, err)
		return nil // skip remove log in order to be able carry on remove backup and log steps
	}

	return nil
}

type TargetAdapter interface {
	Read(string) (any, error)
	Write(string, any) error
	Serialize(any) ([]byte, error)
	UnSerialize([]byte) (any, error)
}

// to read unfinished txn in txn logs and carry on undone txn rollback works
// will only revert change (won't carry out commit)
func Recovery(adpt TargetAdapter) error {
	entries, err := os.ReadDir(".occ/")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var errs []error

	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		txnId, err := decodeTarget(entry.Name())
		if err != nil {
			log.Printf("decode target txnId %s: %v", txnId, err)
			continue
		}

		err = Withdraw(txnId, adpt)
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to recover txn %s: %w", txnId, err,
			))
		}
	}

	return errors.Join(errs...)
}

func discardBackup(txnId, target string) error {
	if err := os.Remove(filepath.Join(".occ/backup", encodeTarget(txnId), encodeTarget(target))); err != nil && !os.IsNotExist(err) {
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
	path := filepath.Join(".occ/backup", encodeTarget(txnId), encodeTarget(target))

	// if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
	// 	return err
	// }

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
	path := filepath.Join(".occ/backup", encodeTarget(txnId), encodeTarget(target))
	return os.ReadFile(path)
}

func loadTargetsToRevert(txnId string) (map[string]bool, bool, error) {
	path := filepath.Join(".occ/journal", encodeTarget(txnId))
	toRevert := make(map[string]bool)

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return toRevert, false, nil
		}
		return nil, false, err
	}
	defer file.Close()

	committed := false
	reader := bufio.NewReader(file)
	var completeBytes int64

	for {
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			// All complete records have been validated before reaching EOF.
			// Remove only the unfinished suffix before rollback appends REVERT.
			if len(line) > 0 {
				if err := truncateLogTail(file, completeBytes); err != nil {
					return nil, false, err
				}
			}
			break
		}
		if err != nil {
			return nil, false, err
		}

		completeBytes += int64(len(line))

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
			toRevert[target] = true
		case "REVERT":
			delete(toRevert, target)
		default:
			return nil, false, fmt.Errorf(
				"unknown log action: %q", action,
			)
		}
	}

	return toRevert, committed, nil
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
	if err := os.MkdirAll(filepath.Join(".occ", encodeTarget(txnId)), 0755); err != nil {
		return err
	}

	path := filepath.Join(".occ", encodeTarget(txnId), "journal")

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

func cleanupLogs(txnId string) error {
	path := filepath.Join(".occ", encodeTarget(txnId), "journal")
	if err := os.RemoveAll(path); err != nil { // won't return error when file already removed (not exist)
		return err
	}
	return nil
}

func cleanupBackups(txnId string) error {
	if err := os.RemoveAll(
		filepath.Join(".occ", encodeTarget(txnId), "backup"); err != nil {
		return err
	}
	return nil
}
