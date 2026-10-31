// Package occ provides optimistic transactions over existing storage targets.
//
// A manager executes commands against a private Context, validates their read
// and write dependencies, and commits staged writes with a disk undo journal.
// Only the outcomes of the successful attempt are returned. Commands must not
// publish external side effects because validation can trigger re-execution.
//
// NewManager locks .occ in the current working directory for the process lifetime
// and completes recovery before returning. Keep that directory associated with
// the same storage and adapter serialization format. Managers in different working
// directories do not coordinate, even if their adapters access the same data.
//
// Adapters must be concurrency-safe and return independent mutable values.
// Transactions support updates to existing targets, not creation or deletion.
// Read errors must be propagated; absence is not a tracked dependency.
// Reads outside this manager are not protected from intermediate writes.
//
// Commit failures trigger rollback. If rollback observes a complete COMMIT
// record, Run reports success even if an earlier journal operation returned an
// error. Rollback failure panics with ErrRollback and leaves recovery state.
// Asynchronous cleanup failures are logged and retried at the next startup.
// The journal does not provide a complete history of successful transactions.
//
// This experimental implementation does not claim power-loss durability:
// journal files are synced, but directory entries are not explicitly synced.
package occ
