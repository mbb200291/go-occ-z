package occ

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Commit(txn Transaction) error {
	for target := range txn.GetWriteSet().All() {
		// backup target
		if err := txn.Backup(target); err != nil {
			_ = Withdraw(txn)
			return err
		}

		// write log first, even target not been write -> idempotent recover by backup
		if err := appendLog(txn.GetID(), "WRITE", target); err != nil {
			_ = Withdraw(txn)
			return err
		}

		// write target
		if err := txn.Write(target); err != nil {
			_ = Withdraw(txn)
			return err
		}
	}

	// if success write, discard all backups
	for target := range txn.GetWriteSet().All() {
		if err := txn.DiscardBackup(target); err != nil {
			log.Printf("failed to discard backup for target %q: %v", target, err)
		}
	}

	// remove txn log
	if err := removeLog(txn.GetID()); err != nil {
		log.Printf("failed to remove txn log %s: %v", txn.GetID(), err)
	}

	return nil
}

func Withdraw(txn Transaction) error {

	// load log files, determine target to revert
	toRevert, err := loadTargetsToRevert(txn.GetID())
	if err != nil {
		return err
	}

	for target := range toRevert {
		// revert target
		if err := txn.Revert(target); err != nil {
			panic(fmt.Sprintf(
				"atomicity broken, failed to revert target %q: %v",
				target,
				err,
			))
		}

		// add revert log after revert had been doen
		if err := appendLog(txn.GetID(), "REVERT", target); err != nil {
			return err
		}

		// discard backup when revert completed
		if err := txn.DiscardBackup(target); err != nil {
			return err
		}
	}

	// remove txn log
	if err := removeLog(txn.GetID()); err != nil {
		log.Printf("failed to remove txn log %s: %v", txn.GetID(), err)
	}

	return nil
}

func loadTargetsToRevert(id string) (map[string]bool, error) {
	path := filepath.Join(".commit", id)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	defer file.Close()

	toRevert := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}

		action := fields[0]
		target := fields[1]

		switch action {
		case "WRITE":
			toRevert[target] = true
		case "REVERT":
			delete(toRevert, target)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return toRevert, nil
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

func removeLog(id string) error {
	path := filepath.Join(".commit", id)

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return nil
}
