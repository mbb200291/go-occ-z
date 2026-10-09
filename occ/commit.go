package occ

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Commit(txn Transaction) error {
	for target := range txn.GetWriteSet().All() {
		// backup target
		ori, err := txn.ReadTarget(target)
		if err != nil {
			if rollbackErr := Withdraw(txn); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
			return err
		}
		oriB, err := txn.Serialize(ori)
		if err != nil {
			if rollbackErr := Withdraw(txn); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
			return err
		}
		if err := saveBackup(txn.GetID(), target, oriB); err != nil {
			if rollbackErr := Withdraw(txn); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
			return err
		}

		// write log first, even target not been write -> idempotent recover by backup
		if err := appendLog(txn.GetID(), "WRITE", target); err != nil {
			if rollbackErr := Withdraw(txn); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
			return err
		}

		// write target
		if err := txn.Write(target, txn.GetOutcome(target)); err != nil {
			if rollbackErr := Withdraw(txn); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
			return err
		}
	}

	// write COMMIT log
	if err := appendLog(txn.GetID(), "COMMIT", ""); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// if success write, discard all backups
	if err := cleanupBackups(txn.GetID()); err != nil {
		log.Printf("failed to remove txn backups %s: %v", txn.GetID(), err)
		return nil // keep log file in order to be able to redo cleanup backups
	}

	// remove txn log
	if err := cleanupLogs(txn.GetID()); err != nil {
		log.Printf("failed to remove txn log %s: %v", txn.GetID(), err)
	}

	return nil
}

// revert target to backup when txn not yet complete write target and remove intermediate products
func Withdraw(txn Transaction) error {

	id := txn.GetID()

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

			ori, err := txn.UnSerialize(orib)
			if err != nil {
				return fmt.Errorf(
					"failed to unserialize backup for %q: %w",
					target, err,
				)
			}

			// revert target
			if err := txn.Write(target, ori); err != nil {
				return fmt.Errorf(
					"failed to revert target %q: %w",
					target, err,
				)
			}

			// add revert log after revert had been doen
			if err := appendLog(id, "REVERT", target); err != nil {
				return err
			}

			// discard backup when revert completed
			if err := discardBackup(id, target); err != nil {
				return err
			}
		}
	}

	// remove txn backups
	if err := cleanupBackups(id); err != nil {
		log.Printf("failed to remove txn backups %s: %v", id, err)
		return nil // skip remove log in order to be able carry on remove backup and log steps
	}

	// remove txn log
	if err := cleanupLogs(id); err != nil {
		log.Printf("failed to remove txn log %s: %v", id, err)
	}

	return nil
}

func discardBackup(txnId, target string) error {
	if err := os.Remove(filepath.Join(".backup", txnId, target)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func saveBackup(txnId, target string, data []byte) error {
	path := filepath.Join(".backup", txnId, target)

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
	path := filepath.Join(".backup", txnId, target)
	return os.ReadFile(path)
}

func loadTargetsToRevert(id string) (map[string]bool, bool, error) {
	path := filepath.Join(".commit", id)
	toRevert := make(map[string]bool)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return toRevert, false, nil
		}
		return nil, false, err
	}
	defer file.Close()

	committed := false
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if committed {
			return nil, false, fmt.Errorf(
				"unexpected log after COMMIT: %q", line,
			)
		}

		if strings.TrimSpace(line) == "COMMIT" {
			committed = true
			continue
		}

		action, target, ok := strings.Cut(line, " ")
		if !ok || target == "" {
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

	if err := scanner.Err(); err != nil {
		return nil, false, err
	}

	return toRevert, committed, nil
}

func appendLog(id, action, target string) error {
	if err := os.MkdirAll(".commit", 0755); err != nil {
		return err
	}

	path := filepath.Join(".commit", id)

	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := fmt.Fprintf(file, "%s %s\n", action, target); err != nil {
		return err
	}

	return file.Sync()
}

func cleanupLogs(id string) error {
	path := filepath.Join(".commit", id)
	if err := os.RemoveAll(path); err != nil { // won't return error when file already removed (not exist)
		return err
	}
	return nil
}

func cleanupBackups(id string) error {
	if err := os.RemoveAll(filepath.Join(".backup", id)); err != nil {
		return err
	}
	return nil
}

// to read unfinished txn in txn logs and carry on undone txn rollback works
// will only revert change (woun't carry out commit)
func TidyUp(id2Txn func(string) (Transaction, error)) error {
	entries, err := os.ReadDir(".commit")
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

		id := entry.Name()
		var txn Transaction
		txn, err = id2Txn(id)
		if err == nil {
			err = Withdraw(txn)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to recover txn %s: %w", id, err,
			))
		}
	}

	return errors.Join(errs...)
}
