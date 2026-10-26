<!-- pohw-badges:start -->
[![PoHW Editing Evidence Score](https://raw.githubusercontent.com/mbb200291/go-occ-z/pohw-badges/editing.svg)](https://github.com/mbb200291/go-occ-z/blob/pohw-badges/REPORT.md) [![PoHW Monitored Coverage](https://raw.githubusercontent.com/mbb200291/go-occ-z/pohw-badges/coverage.svg)](https://github.com/mbb200291/go-occ-z/blob/pohw-badges/REPORT.md)
<!-- pohw-badges:end -->

# go-occ-z

Optimistic concurrency control for Go, using backward validation.
Based on [CMU Lecture #18](https://15445.courses.cs.cmu.edu/fall2024/schedule.html).

Under development. Not ready for production.

## Usage

Implement `occ.TargetAdapter` for storage and `occ.Command` for operations.
One `Run` executes the supplied commands as one transaction.

```go
import "github.com/mbb200291/go-occ-z/occ"

manager, err := occ.NewManager(adapter)
if err != nil {
    return err
}
outcomes, err := manager.Run(commands) // commands is []occ.Command
if err != nil {
    return err
}
// Use outcomes.
```

Share the manager across goroutines. Validation conflicts will retried internaly by occ. Retry parameters can be tuned by `occ.RetryOptions`.
Command execution errors will interrupt execution process and returned without retrying.

## Implement TargetAdapter

```go
type TargetAdapter interface {
    Read(target string) (any, error)
    Write(target string, value any) error
    Serialize(value any) ([]byte, error)
    UnSerialize(data []byte) (any, error)
}
```

- Choose stable target keys, such as record IDs or line numbers. Commands touching the same data must use the same key.
- `Read` returns the current value. Copy mutable data, including nested maps and slices, so commands cannot change shared storage through a reference.
- `Write` function will finalize data, such as save data to file. It will be called when commit or rollback.
- `Serialize` encodes data to a bytes. It will be called during backup data before finalize. `UnSerialize` must restore a value that `Write` accepts, including after a process restart.
- Methods may run concurrently. Protect shared adapter state and preserve unrelated targets when writing. Complete the required storage flush before returning success.

OCC stores the returned backup bytes; it does not require JSON.

Add a compile-time check to your implementation:

```go
var _ occ.TargetAdapter = (*MyAdapter)(nil)
```

## Implement Command

Declare all targets before execution. Read-only commands return `true` from
`IsReadOnly`; write-only commands return `true` from `IsWriteOnly`. For a
read-modify-write operation, both return `false`. Do not set both to `true`.
Write-only targets are not loaded, so set their values before commit.

This example assumes the adapter returns `int64` values. It needs `fmt` and
`github.com/mbb200291/go-occ-z/occ` imports.

```go
type Increment struct {
    Target string
}

func (Increment) IsReadOnly() bool     { return false }
func (Increment) IsWriteOnly() bool    { return false }
func (c Increment) GetTargets() []string { return []string{c.Target} }

func (c Increment) Execute(ctx occ.Context) error {
    value, ok := ctx.GetData(c.Target).(int64)
    if !ok {
        return fmt.Errorf("target %q: expected int64", c.Target)
    }
    if value == int64(1<<63-1) {
        return fmt.Errorf("target %q: counter overflow", c.Target)
    }
    value++
    ctx.SetData(c.Target, value)
    ctx.AppendOutcome(value)
    return nil
}

var _ occ.Command = Increment{}
```

Pass it to `manager.Run([]occ.Command{Increment{Target: "counter"}})`.
Only use Context for transaction data. Do not access undeclared targets, retain
the context after execution, or mutate command inputs while Run is active.
Commands may execute more than once: avoid direct file writes, messages, or
other side effects that rollback cannot undo.

## Retry settings

Defaults: 32 retries, a 1ms initial delay, a 100ms maximum delay, jitter off.
Pass one optional Config to change them (import `time`):

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

A nil Retry uses defaults. Otherwise, supply all three numeric fields.
`MaxRetries: 0` disables retries; delays must satisfy `0 < BaseDelay <= MaxDelay`.
Jitter chooses a wait between 50% and 100% of each exponential delay.

## Storage and lifetime

- Initialization recovers pending transactions from `.occ` in the working directory. Keep that directory associated with the same storage across restarts.
- Only one manager can own a `.occ` directory. Another gets `ErrDirectoryInUse`. The lock lasts until process exit.
- Do not delete journals or the lock file, or change the working directory while using the manager.
- Different working directories have separate locks. occ-z Won't protect data from been modified by other applications.
