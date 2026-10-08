package JsonlOcc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/mbb200291/go-occ-z/occ"
	"github.com/mbb200291/go-occ-z/occ/transaction"
)

func fixture(t *testing.T, data string) string {
	t.Helper()
	t.Chdir(t.TempDir())
	path := filepath.Join(t.TempDir(), "records.jsonl")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadUpdateAndPreserveOtherLines(t *testing.T) {
	path := fixture(t, "{\"name\":\"Ada\",\"items\":[{\"price\":5}],\"id\":9007199254740993}\r\n  {\"name\":\"Bob\"}")
	engine, err := NewEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := engine.Execute([]transaction.Command{
		NewRead(1, "$.name"), NewUpdate(1, "$.items[0].price", 12), NewRead(1, "$.items[0].price"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, []any{"Ada", json.Number("12"), json.Number("12")}) {
		t.Fatalf("outcomes: %#v", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"id\":9007199254740993,\"items\":[{\"price\":12}],\"name\":\"Ada\"}\r\n  {\"name\":\"Bob\"}"
	if string(data) != want {
		t.Fatalf("file: %q, want %q", data, want)
	}
}

func TestPrivateWorkspaceDoesNotWriteBeforeCommit(t *testing.T) {
	original := "{\"x\":1}\n"
	path := fixture(t, original)
	ctx := NewJsonlContext(path)
	if err := ctx.Load("1"); err != nil {
		t.Fatal(err)
	}
	if err := NewUpdate(1, "$.x", 2).Execute(ctx); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatal("command modified shared file before Write")
	}
	records := ctx.GetRecords([]uint{1})
	records[0].(map[string]any)["x"] = "external mutation"
	if err := ctx.Write("1"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "{\"x\":2}\n" {
		t.Fatalf("private record corrupted: %s", data)
	}
}

func TestInvalidCommandsLeaveFileUnchanged(t *testing.T) {
	for _, tt := range []struct {
		name, data string
		cmd        transaction.Command
	}{
		{"zero line", "{\"x\":1}\n", NewRead(0, "$")},
		{"missing line", "{\"x\":1}\n", NewRead(2, "$")},
		{"missing field", "{\"x\":1}\n", NewRead(1, "$.missing")},
		{"wildcard", "{\"x\":1}\n", NewRead(1, "$.*")},
		{"negative index", "{\"x\":[1]}\n", NewUpdate(1, "$.x[-1]", 2)},
		{"array bounds", "{\"x\":[1]}\n", NewUpdate(1, "$.x[1]", 2)},
		{"malformed json", "not json\n", NewRead(1, "$")},
		{"multiple json values", "{} {}\n", NewRead(1, "$")},
		{"blank line", "\n", NewRead(1, "$")},
		{"invalid value", "{\"x\":1}\n", NewUpdate(1, "$.x", make(chan int))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := fixture(t, tt.data)
			engine, err := NewEngine(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Execute([]transaction.Command{tt.cmd}); err == nil {
				t.Fatal("expected error")
			}
			data, _ := os.ReadFile(path)
			if string(data) != tt.data {
				t.Fatalf("failed transaction changed file: %s", data)
			}
		})
	}
}

func TestReadOnlyAndRootReplacement(t *testing.T) {
	path := fixture(t, " {\"x\":null} \n")
	engine, err := NewEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := engine.Execute([]transaction.Command{NewRead(1, "$.x")})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0] != nil {
		t.Fatalf("null: %#v", out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != " {\"x\":null} \n" {
		t.Fatal("read-only transaction rewrote file")
	}
	_, err = engine.Execute([]transaction.Command{NewUpdate(1, "$", []any{true, "value"})})
	if err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "[true,\"value\"]\n" {
		t.Fatalf("root: %s", data)
	}
}

func TestWritesToDifferentLinesPreserveBothChanges(t *testing.T) {
	path := fixture(t, "{\"x\":1}\n{\"x\":2}\n")
	a, b := NewJsonlContext(path), NewJsonlContext(path)
	if err := a.Load("1"); err != nil {
		t.Fatal(err)
	}
	if err := b.Load("2"); err != nil {
		t.Fatal(err)
	}
	if err := NewUpdate(1, "$.x", 3).Execute(a); err != nil {
		t.Fatal(err)
	}
	if err := NewUpdate(2, "$.x", 4).Execute(b); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, v := range []struct {
		ctx  *JsonlContext
		line string
	}{{a, "1"}, {b, "2"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := v.ctx.Write(v.line); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	data, _ := os.ReadFile(path)
	if string(data) != "{\"x\":3}\n{\"x\":4}\n" {
		t.Fatalf("lost unrelated update: %s", data)
	}
}

// Catch dropping committed history: a transaction that read an old value must
// reject a writer committed before its later validation.
func TestCommittedHistoryRejectsStaleTransaction(t *testing.T) {
	path := fixture(t, "{\"x\":1}\n")
	engine, err := NewEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := transaction.NewTransaction([]transaction.Command{NewUpdate(1, "$.x", 3)}, NewJsonlContext(path))
	stale.SetReadTime(occ.NextTimestamp())
	engine.ongoingTxns.Add(stale)
	defer engine.ongoingTxns.Remove(stale)
	if err := stale.Read(); err != nil {
		t.Fatal(err)
	}
	if err := stale.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Execute([]transaction.Command{NewUpdate(1, "$.x", 2)}); err != nil {
		t.Fatal(err)
	}
	stale.SetValidateTime(occ.NextTimestamp())
	if occ.Validate(stale, engine.prevTxns, stale.GetReadTime()) {
		t.Fatal("committed writer missing from validation history")
	}
}

func TestUpdateMetadataAndContextErrors(t *testing.T) {
	cmd := NewUpdate(1, "$.x", 2)
	txn := transaction.NewTransaction([]transaction.Command{cmd}, nil)
	if !txn.GetReadSet().Contains("1") || !txn.GetWriteSet().Contains("1") {
		t.Fatal("update must load and validate read dependency")
	}
	ctx := NewJsonlContext(fixture(t, "{}\n"))
	if err := cmd.Execute(ctx); err == nil {
		t.Fatal("unloaded record accepted")
	}
	if _, err := ctx.GetOutcomes(); err == nil {
		t.Fatal("context did not retain command error")
	}
	if err := cmd.Execute(&transaction.ContextBase{}); err == nil {
		t.Fatal("wrong context type accepted")
	}
}

func TestRevertAfterErrorPreservesOtherLines(t *testing.T) {
	path := fixture(t, " {\"x\":1} \r\n{\"x\":2}")
	ctx := NewJsonlContext(path)
	for _, step := range []func() error{
		func() error { return ctx.Load("1") },
		func() error { return ctx.Backup("1") },
		func() error { return NewUpdate(1, "$.x", 3).Execute(ctx) },
		func() error { return ctx.Write("1") },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	other := NewJsonlContext(path)
	if err := other.Load("2"); err != nil {
		t.Fatal(err)
	}
	other.Update(2, map[string]any{"x": 4})
	if err := other.Write("2"); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Write("0"); err == nil {
		t.Fatal("expected error")
	}
	for range 2 {
		if err := ctx.Revert("1"); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != " {\"x\":1} \r\n{\"x\":4}" {
		t.Fatalf("rollback: %q", data)
	}
	if err := ctx.DiscardBackup("1"); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Revert("1"); err == nil {
		t.Fatal("missing backup accepted")
	}
}
