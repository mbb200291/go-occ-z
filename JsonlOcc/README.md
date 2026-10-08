# JSONL OCC

A transaction context loads records into an in-memory private workspace. Commands
read and update that workspace; `occ.Execute` validates before writing it back.
The existing file must contain one JSON value per physical line. Line numbers
start at **1**; blank lines are not skipped and cannot be loaded as records.

```go
import (
    jsonlocc "github.com/mbb200291/go-occ-z/JsonlOcc"
    "github.com/mbb200291/go-occ-z/occ/transaction"
)

engine, err := jsonlocc.NewEngine("records.jsonl")
if err != nil {
    return err
}

outcomes, err := engine.Execute([]transaction.Command{
    jsonlocc.NewRead(1, "$.name"),
    jsonlocc.NewUpdate(1, "$.items[0].price", 12),
    jsonlocc.NewRead(1, "$.items[0].price"),
})
if err != nil {
    return err
}
// outcomes contains the read value, the updated value, and the final read value.
_ = outcomes
```

Share **one Engine per file** between concurrent callers. Each Execute creates
its own context. Multiple independently created engines do not share OCC history.
External programs changing the file do not participate in OCC validation.

## Paths and results

- `$` selects or replaces the whole record, including scalar or null values.
- `$.name`, `$.profile.name` and `$.items[0].price` select existing fields.
- `$[0]` selects an item in a root array.
- Updates require the path to exist; they do not create fields or extend arrays.
- Wildcards, filters, recursive descent and quoted field names are unsupported.
- Each command adds one outcome in command order. JSON numbers are returned as
  `json.Number`, preserving integers larger than float64 can represent.
- Missing lines, invalid JSON, invalid paths, unsupported values and I/O failures
  return errors. A command failure before validation leaves the file untouched.

Updated lines are encoded as compact JSON. Unrelated lines are preserved byte for
byte, including their whitespace. Existing LF/CRLF endings and a missing final
newline are preserved. Create/delete commands are not implemented.

## Context API

`NewJsonlContext(path)` creates a private workspace. `Load("1")` copies a line
once, so repeated loads within that context retain the private version.
`GetRecords([]uint{1})` returns independent JSON copies. `Update(1, value)` replaces
an already-loaded private record; errors are retained by `GetOutcomes()`.
`Write("1")` installs the private record in the file. Prefer Engine.Execute for
transactions; direct context operations bypass OCC validation.

## Current limits

Individual file operations use an in-process mutex shared by canonical path.
Each operation reads the whole file; writes use a temporary file and rename to
avoid exposing a partially rewritten file. This is intended for small files.
Replacement preserves permission bits but changes the inode (hard links and
extended metadata are not preserved), and requires a writable parent directory.

Backups retain the original line bytes in memory. Rollback restores only those
lines, preserving unrelated updates. Backups do not survive process termination;
this does not provide crash recovery, cross-process coordination, or atomic
visibility of multi-line commits. Share one Engine per file for OCC validation.
The generic OCC history registration/purge synchronization remains a separate
limitation of the core engine.

Run the package checks with:

```sh
go test -race ./JsonlOcc ./occ/...
go vet ./JsonlOcc ./occ/...
```
