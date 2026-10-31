<!-- pohw-badges:start -->
[![PoHW Editing Evidence Score](https://raw.githubusercontent.com/mbb200291/go-occ-z/pohw-badges/editing.svg)](https://github.com/mbb200291/go-occ-z/blob/pohw-badges/REPORT.md) [![PoHW Monitored Coverage](https://raw.githubusercontent.com/mbb200291/go-occ-z/pohw-badges/coverage.svg)](https://github.com/mbb200291/go-occ-z/blob/pohw-badges/REPORT.md)
<!-- pohw-badges:end -->

# go-occ-z

[![Unit tests](https://github.com/mbb200291/go-occ-z/actions/workflows/test.yml/badge.svg?branch=dev)](https://github.com/mbb200291/go-occ-z/actions/workflows/test.yml)

Optimistic concurrency control for Go, using backward validation.
Based on [CMU Lecture #18](https://15445.courses.cs.cmu.edu/fall2024/schedule.html).

Experimental; not ready for production. Requires Go 1.25 or newer.

```sh
go get github.com/mbb200291/go-occ-z/occ
```

## Quick start

Implement a `TargetAdapter` for your storage, then share one manager across goroutines.
Each `Run` executes its commands as one transaction. Commands read and write through
Context and return their result directly.

This example expects an existing `counter` target containing an `int64`:

```go
import (
    "fmt"

    "github.com/mbb200291/go-occ-z/occ"
)

type Increment struct{ Target string }

func (c Increment) Execute(ctx occ.Context) (any, error) {
    data, err := ctx.GetData(c.Target)
    if err != nil {
        return nil, err
    }
    value, ok := data.(int64)
    if !ok || value == int64(1<<63-1) {
        return nil, fmt.Errorf("target %q: invalid counter or overflow", c.Target)
    }
    ctx.SetData(c.Target, value+1)
    return value + 1, nil
}

func incrementCounter(adapter occ.TargetAdapter) error {
    manager, err := occ.NewManager(adapter)
    if err != nil {
        return err
    }
    outcomes, err := manager.Run([]occ.Command{Increment{Target: "counter"}})
    if err != nil {
        return err
    }
    fmt.Println(outcomes[0])
    return nil
}
```

Create the manager once and reuse it. Targets can be chosen during execution;
`GetData` and `SetData` track the reads and writes. Results follow command order.
Return `nil, nil` when a command has no result.

## Implement TargetAdapter

```go
type TargetAdapter interface {
    Read(target string) (any, error)
    Write(target string, value any) error
    Serialize(value any) ([]byte, error)
    UnSerialize(data []byte) (any, error)
}
```

- Use the same target key for the same data. `Read` must return independent data,
  including copies of nested maps and slices.
- Methods may run concurrently. Writes must preserve unrelated targets and finish
  the required storage flush before returning success.
- If `Write` retains mutable input, copy it so later changes to an outcome cannot
  mutate storage outside OCC.
- `Write` handles both commit and rollback. Serialization must preserve the original
  value for restoration after a restart; the byte format is up to the adapter.

## Options

Validation conflicts retry automatically, then fall back to one exclusive attempt.
Command and storage errors are returned without retrying. Use `manager.RunExclusive(commands)`
to wait for active calls and execute exclusively from the start.

Defaults: 32 retries, 1ms initial delay, 100ms maximum delay, jitter off.
To override them, pass an optional config (import `time`):

```go
manager, err := occ.NewManager(adapter, occ.Config{
    Retry: &occ.RetryOptions{
        MaxRetries: 5,
        BaseDelay:  time.Millisecond,
        MaxDelay:   100 * time.Millisecond,
        Jitter:     true,
    },
})
```

Provide all numeric fields when overriding. `MaxRetries: 0` goes straight to the
exclusive attempt after the first conflict. Delays require `0 < BaseDelay <= MaxDelay`.

## Notes

- Only existing targets can be updated. Creating or deleting files or records is not supported.
- Commands may run more than once. Use Context for transaction data, propagate read
  errors, and avoid external side effects. Do not retain the context, modify its
  dependency sets, or call the same manager recursively from a command.
- Startup recovers pending transactions from `.occ` in the working directory.
  Keep it tied to the same storage. Do not delete its files or change the working
  directory while the manager is in use.
- One manager owns each `.occ` directory until process exit; another gets
  `ErrDirectoryInUse`. Separate directories have separate locks, so prevent other
  managers or applications from independently modifying the same data.

## Errors and recovery

Use `errors.Is` for public errors and the underlying adapter/command error.
`ErrIniManager` covers configuration, directory locking, and startup recovery;
`ErrIniTxn` covers empty command lists and nil commands. Do not pass typed-nil
adapters or commands.

Command errors return `ErrExecute` without commit. Read errors propagated from
`Context.GetData` additionally carry `ErrRead`. Commit failures trigger rollback and normally return `ErrCommit`. If
rollback finds a complete `COMMIT` record, the call instead returns success.
Rollback failure **panics** with `ErrRollback`; stop using the storage and recover
on restart. Do not catch this panic and continue serving transactions.

Cleanup failures after commit are logged; startup retries cleanup. Successful
transaction IDs are not a durable history. The current implementation syncs files
but not parent directories, so it does **not** guarantee power-loss durability.
Readers outside the manager can observe intermediate writes. Recovery requires the
same adapter, storage, and serialization format as the original transaction.

## Tests

```sh
go test -race -count=1 -timeout=5m ./...
go vet ./...
```

The unit-test badge reports the `dev` workflow status, not test coverage. CI tests
Go 1.25 and stable on Linux, macOS, and Windows, and uploads coverage reports.
Tests cover validation, conflict retries, exclusive fallback, concurrent updates,
rollback, interrupted recovery, and directory ownership. Benchmark results are
not a substitute for these correctness tests.

[Release notes](CHANGELOG.md) · [MIT license](LICENSE)
