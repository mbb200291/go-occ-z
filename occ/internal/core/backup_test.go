package core

import (
	"bytes"
	"os"
	"testing"
)

func TestBackupFileHandler(t *testing.T) {
	t.Chdir(t.TempDir())
	h := NewBackupFileHandler("txn")
	data := bytes.Repeat([]byte{0, 255, '\n'}, 30000)
	if err := h.SaveBackup("file\nname", true, data); err != nil {
		t.Fatal(err)
	}
	if err := h.SaveBackup("new", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := h.Sync(); err != nil {
		t.Fatal(err)
	}
	// Reopen through a fresh handler, as recovery does.
	var entries []*BackupEntry
	for entry, err := range NewBackupFileHandler("txn").LoadBackups() {
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 2 || entries[0].Target != "file\nname" || !entries[0].Exist || !bytes.Equal(entries[0].Content, data) || entries[1].Exist {
		t.Fatalf("unexpected backups: %v", entries)
	}
	f, err := os.OpenFile(h.path(), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"Target":`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, err := range h.LoadBackups() {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("got %d complete records", count)
	}
	for _, err := range h.LoadBackups() {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if err := h.RemoveBackups(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(transactionDir("txn")); !os.IsNotExist(err) {
		t.Fatalf("directory remains: %v", err)
	}
	if err := h.RemoveBackups(); err != nil {
		t.Fatal(err)
	}
}

func TestBackupReadAndSyncErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	h := NewBackupFileHandler("txn")
	if err := h.Sync(); err == nil {
		t.Fatal("sync of missing backup succeeded")
	}
	for _, err := range h.LoadBackups() {
		t.Fatalf("missing backup should be empty: %v", err)
	}
	if err := h.SaveBackup("x", true, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.path(), []byte("broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, err := range h.LoadBackups() {
		if err == nil {
			t.Fatal("malformed record accepted")
		}
		seen = true
	}
	if !seen {
		t.Fatal("missing decode error")
	}
}
