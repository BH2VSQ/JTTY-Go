# JTTY-Go v5.36-fixed8

## Test and radio configuration fixes

This maintenance release fixes two test failures reported against v5.36-fixed7 and removes one unnecessary radio-settings migration flag.

### 1. Selected-radio VOX configuration is no longer reported as migrated

`normalizeLoadedSettings()` now keeps a valid Hamlib radio selection with `PTTMethod=VOX` unchanged. A missing `RigName`/`None` label does not by itself mark the settings dirty because Hamlib enumeration is responsible for refreshing the display name.

### 2. Correct rigctld argument test semantics

The rigctld command line is validated using normal executable argument pairs such as:

- `-m` + `1020`
- `-r` + `COM3`
- `-s` + `4800`

This matches the actual `exec.Command()` argument representation. The implementation itself was already passing `-s` and `4800` as two separate arguments; the previous test incorrectly looked for a single literal argument `-s 4800`.

### 3. No UI layout changes

This release only fixes the reported test/migration issues. Existing QSO TX-only frequency display, UTC ordering, startup macro-name loading, and Hamlib radio-list behavior remain unchanged from v5.36-fixed7.
