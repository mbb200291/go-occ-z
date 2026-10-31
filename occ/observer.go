package occ

import "github.com/mbb200291/go-occ-z/occ/internal/core"

// TraceEvent contains an attempt ID, logical timestamps, and copied target sets.
// Zero timestamps are unset; OtherID identifies a conflicting or purged transaction.
type TraceEvent = core.TraceEvent

// Observer runs synchronously, possibly concurrently across Run and RunExclusive calls.
// Keep it short; do not reenter a manager or mutate storage. Panics are logged
// and contained. Events are diagnostic points, not atomic container snapshots.
type Observer = core.Observer
