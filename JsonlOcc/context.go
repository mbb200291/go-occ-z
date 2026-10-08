package JsonlOcc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/mbb200291/go-occ-z/occ/transaction"
)

// JsonlContext is a transaction's private workspace. Use one context per
// transaction; its records and outcomes are not shared between goroutines.
type JsonlContext struct {
	path     string
	fileMu   *sync.Mutex
	outcomes []any
	err      error
	records  map[uint]any
	backups  map[uint][]byte
}

// File access is coordinated between contexts using the same canonical path.
// This protects individual file operations, not multi-line transaction commits.
var fileLocks sync.Map

func canonicalPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("JSONL file path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

func NewJsonlContext(path string) *JsonlContext {
	ctx := &JsonlContext{records: make(map[uint]any), backups: make(map[uint][]byte)}
	normalized, err := canonicalPath(path)
	if err != nil {
		ctx.err = err
		return ctx
	}
	ctx.path = normalized
	lock, _ := fileLocks.LoadOrStore(normalized, &sync.Mutex{})
	ctx.fileMu = lock.(*sync.Mutex)
	return ctx
}

func parseLine(target string) (uint, error) {
	n, err := strconv.ParseUint(target, 10, strconv.IntSize)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != target {
		return 0, fmt.Errorf("invalid JSONL target %q: expected a positive decimal line number", target)
	}
	return uint(n), nil
}

func (ctx *JsonlContext) fail(err error) error {
	if ctx.err == nil {
		ctx.err = err
	}
	return err
}

func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values in one line")
		}
		return nil, err
	}
	return value, nil
}

func cloneJSON(value any) (any, error) {
	data, err := json.Serialize(value)
	if err != nil {
		return nil, err
	}
	return decodeJSON(data)
}

func linesOf(data []byte) [][]byte {
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (ctx *JsonlContext) Load(target string) error {
	if ctx.err != nil {
		return ctx.err
	}
	line, err := parseLine(target)
	if err != nil {
		return ctx.fail(err)
	}
	// Reusing the loaded copy ensures reads within a transaction are repeatable.
	if _, ok := ctx.records[line]; ok {
		return nil
	}
	err = ctx.editLine(line, func(raw []byte) ([]byte, error) {
		value, err := decodeJSON(raw)
		if err == nil {
			ctx.records[line] = value
		}
		return nil, err
	})
	return ctx.fail(err)
}

func (ctx *JsonlContext) Write(target string) error {
	if ctx.err != nil {
		return ctx.err
	}
	line, err := parseLine(target)
	if err != nil {
		return ctx.fail(err)
	}
	value, ok := ctx.records[line]
	if !ok {
		return ctx.fail(fmt.Errorf("JSONL line %d is not loaded", line))
	}
	encoded, err := json.Serialize(value)
	if err != nil {
		return ctx.fail(err)
	}
	return ctx.fail(ctx.editLine(line, func(old []byte) ([]byte, error) {
		if bytes.HasSuffix(old, []byte("\r\n")) {
			encoded = append(encoded, '\r', '\n')
		} else if bytes.HasSuffix(old, []byte("\n")) {
			encoded = append(encoded, '\n')
		}
		return encoded, nil
	}))
}

func (ctx *JsonlContext) Backup(target string) error {
	if ctx.err != nil {
		return ctx.err
	}
	line, err := parseLine(target)
	if err != nil {
		return ctx.fail(err)
	}
	if _, ok := ctx.backups[line]; ok {
		return nil
	}
	return ctx.fail(ctx.editLine(line, func(raw []byte) ([]byte, error) {
		ctx.backups[line] = bytes.Clone(raw)
		return nil, nil
	}))
}

func (ctx *JsonlContext) Revert(target string) error {
	line, err := parseLine(target)
	if err != nil {
		return err
	}
	raw, ok := ctx.backups[line]
	if !ok {
		return fmt.Errorf("JSONL line %d has no backup", line)
	}
	// Rollback must work even when the context has retained a write error.
	return ctx.editLine(line, func([]byte) ([]byte, error) { return raw, nil })
}

// A nil replacement reads a line without rewriting the file.
func (ctx *JsonlContext) editLine(line uint, edit func([]byte) ([]byte, error)) error {
	ctx.fileMu.Lock()
	defer ctx.fileMu.Unlock()
	data, err := os.ReadFile(ctx.path)
	if err != nil {
		return err
	}
	lines := linesOf(data)
	if line > uint(len(lines)) {
		return fmt.Errorf("JSONL line %d does not exist", line)
	}
	replacement, err := edit(lines[line-1])
	if err != nil || replacement == nil {
		return err
	}
	lines[line-1] = replacement
	info, err := os.Stat(ctx.path)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(ctx.path), ".jsonl-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := file.Write(bytes.Join(lines, nil)); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), ctx.path)
}

func (ctx *JsonlContext) record(line uint) (any, error) {
	value, ok := ctx.records[line]
	if !ok {
		return nil, ctx.fail(fmt.Errorf("JSONL line %d is not loaded", line))
	}
	copy, err := cloneJSON(value)
	if err != nil {
		return nil, ctx.fail(err)
	}
	return copy, nil
}

// GetRecords returns independent copies in the requested order. Missing records
// return nil and retain an error available from GetOutcomes.
func (ctx *JsonlContext) GetRecords(lines []uint) []any {
	values := make([]any, 0, len(lines))
	for _, line := range lines {
		value, err := ctx.record(line)
		if err != nil {
			return nil
		}
		values = append(values, value)
	}
	return values
}

// Update replaces a loaded record in the private workspace only.
func (ctx *JsonlContext) Update(line uint, value any) {
	if ctx.err != nil {
		return
	}
	if _, ok := ctx.records[line]; !ok {
		ctx.fail(fmt.Errorf("JSONL line %d is not loaded", line))
		return
	}
	copy, err := cloneJSON(value)
	if err != nil {
		ctx.fail(err)
		return
	}
	ctx.records[line] = copy
}

func (ctx *JsonlContext) appendOutcome(value any) error {
	copy, err := cloneJSON(value)
	if err != nil {
		return ctx.fail(err)
	}
	ctx.outcomes = append(ctx.outcomes, copy)
	return nil
}

func (ctx *JsonlContext) GetOutcomes() ([]any, error) {
	result := make([]any, len(ctx.outcomes))
	for i, value := range ctx.outcomes {
		copy, err := cloneJSON(value)
		if err != nil {
			return nil, err
		}
		result[i] = copy
	}
	return result, ctx.err
}

// Ensure the JSONL context supports the generic transaction lifecycle.
var _ transaction.Context = (*JsonlContext)(nil)
