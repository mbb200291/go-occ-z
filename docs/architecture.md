# OCC architecture

The diagrams follow the current `dev` implementation. The observer extension is
published separately as `v0.1.0-observe.1`.

![OCC overview](assets/architecture.svg)

The overview groups two ordered containers under Transaction Container. Reading
and computation are shown separately for clarity. Commands load data on demand
through Context. Registration in Active occurs before reads, and registration in
History occurs before validation.

## Components

```mermaid
flowchart LR
    subgraph app[Application]
        caller[Caller]
        cmd[Command.Execute]
        adapter[TargetAdapter]
        storage[(Target storage)]
    end

    subgraph api[occ]
        manager[Manager: Run / RunExclusive]
        startup[NewManager: lock and recovery]
    end

    subgraph impl[occ/internal]
        txn[Transaction: ID and timestamps]
        ctx[Context: private values and dependency sets]
        ongoing[ongoingTxn: ordered by ReadTime]
        previous[prevTxn: ordered by ValidateTime]
    end

    subgraph core[occ/internal/core]
        process[Process / RetryProcess]
        validate[Validate]
        commit[Commit / Withdraw]
        recovery[Recovery]
    end

    journal[(.occ: lock, journals, backups)]

    caller --> startup
    startup --> manager
    startup --> recovery
    startup -->|process-lifetime lock| journal
    caller --> manager
    manager -->|fresh attempt| txn
    manager -->|fresh attempt| ctx
    manager --> process
    process -->|Execute in command order| cmd
    cmd -->|GetData / SetData| ctx
    ctx -->|read on cache miss| adapter
    txn -->|read and write sets| ctx
    process -->|register / remove| ongoing
    process -->|register before validation| previous
    process --> validate
    validate -->|compare and purge history| previous
    process -->|GetMin for purge threshold| ongoing
    process --> commit
    commit -->|staged values| txn
    commit -->|read, serialize, write, restore| adapter
    commit --> journal
    recovery -->|Withdraw each transaction| commit
    adapter --> storage
```

- `GetData` caches successful storage reads and records their targets in the read
  set. `SetData` stages values and adds targets to the write set. The adapter must
  return independent mutable data.
- `ongoingTxn` contains attempts from ReadTime registration until `Process` exits.
  `prevTxn` contains validating attempts and retained completed history. A conflict
  removes its own entry. Validation purges visited entries whose WriteTime is
  earlier than every ongoing ReadTime.
- `runMu.RLock` lets ordinary `Run` calls overlap, including their retries.
  `RunExclusive` uses `runMu.Lock` for one attempt. The short `muR` and `muV`
  critical sections protect timestamp allocation and registration. Validation
  and commit run after those locks are released.

## Transaction flow

```mermaid
flowchart TD
    run[Run: acquire shared runMu lock] --> fresh
    exclusive[RunExclusive: acquire exclusive runMu lock] --> fresh
    fresh[Create fresh Transaction and Context] --> init{Valid commands?}
    init -->|No| initerr[Return ErrIniTxn]
    init -->|Yes| read[Set ReadTime and register in ongoingTxn]
    read --> execute[Execute commands against private Context]
    execute -->|Command or read error| execerr[Return ErrExecute with underlying error]
    execute -->|Success| stamp[Set ValidateTime and register in prevTxn]
    stamp --> validate{Validate against earlier transactions}
    validate -->|Conflict| remove[Remove from prevTxn and ongoingTxn]
    remove --> mode{Attempt mode}
    mode -->|Ordinary, retries remain| delay[Exponential backoff with optional jitter]
    delay --> fresh
    mode -->|Ordinary, retries exhausted| unlock[Release shared runMu lock]
    unlock --> exclusive
    mode -->|Exclusive| conflict[Return ErrConflict]

    validate -->|Pass| writes{Write set empty?}
    writes -->|Yes| success[Set WriteTime and return outcomes]
    writes -->|No| backup[For each target: read and save original value]
    backup --> logwrite[Append and sync WRITE record]
    logwrite --> write[Adapter.Write staged value]
    write -->|Next target| backup
    write -->|All targets written| committed[Append and sync COMMIT record]
    committed -->|Success| cleanup[Schedule asynchronous cleanup]
    cleanup --> success

    backup -->|Error| withdraw[Withdraw: inspect journal]
    logwrite -->|Error| withdraw
    write -->|Error| withdraw
    committed -->|Error| withdraw
    withdraw -->|Journal read or parse error| panicnode
    withdraw --> found{Complete COMMIT found?}
    found -->|Yes| success
    found -->|No| rollback[Restore pending targets from backup and record REVERT]
    rollback -->|Success, including cleanup| failed[Set WriteTime and return ErrCommit]
    rollback -->|Error| panicnode[Panic with ErrRollback]
```

Each attempt removes itself from `ongoingTxn` when `Process` exits, including
error and panic paths. The diagram shows that removal explicitly on the retry
path. Returning or panicking also releases the manager's run lock.

ReadTime marks the start of the attempt, before storage reads. ValidateTime is set
after private execution. WriteTime stays at a sentinel value until writes or
rollback finish. Read-only success also sets WriteTime.

Only a bare `ErrConflict` triggers retry. The same Command objects execute again
with fresh transaction state. The exclusive fallback executes once. Successful
outcomes are returned in command order.

### Journal and recovery

Each transaction uses `.occ/<txn-id>/journal` and `.occ/<txn-id>/backup/`.
Commit processes one target at a time: backup, WRITE record, then adapter write.
After all targets are written, it records COMMIT and schedules cleanup.

`Withdraw` preserves data when it finds a complete COMMIT record. Otherwise it
restores targets with outstanding WRITE records, appends REVERT, and cleans up.
An incomplete journal suffix is truncated before appending recovery records.
Directories with no journal are also cleaned up. If COMMIT is found after a
commit error, `Process` returns success even if cleanup needs another attempt.

At startup, `NewManager` acquires the `.occ` lock and runs `Recovery`, which calls
`Withdraw` for each transaction directory. Initialization errors release the lock.
A successfully initialized manager retains it until process exit. After a rollback
panic, restart with the same storage and working directory to run recovery.

The adapter controls storage writes and flushing. Core syncs backup and journal
files. Parent directory synchronization remains pending, so power-loss durability
requires further work.

## Source

- [Manager and configuration](../occ/main.go)
- [Private Context](../occ/internal/context.go)
- [Transaction state](../occ/internal/transaction.go)
- [Execution and retries](../occ/internal/core/process.go)
- [Validation and history cleanup](../occ/internal/core/validate.go)
- [Commit, rollback, and recovery](../occ/internal/core/commit.go)
- [Directory ownership lock](../occ/lock.go)
