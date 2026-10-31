# Changelog

## Unreleased

- Add unit and integration tests for validation, retries, transaction outcomes,
  concurrent updates, rollback, recovery, and cross-process directory locking.
- Add a unit-test CI workflow and README status badge, with coverage artifacts.
- Document adapter isolation, retry side effects, errors, directory ownership,
  existing-target-only updates, and recovery/durability limitations.
- Declare the missing flock dependency and require Go 1.25 (the minimum required
  by the selected flock/x/sys versions).
- Reject nil commands with ErrIniTxn instead of panicking.

No stable release or production durability guarantee is implied by these changes.
Before tagging, run the CI matrix and verify installation using the intended tag.
