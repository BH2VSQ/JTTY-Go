# JTTY-Go v5.36-fixed16

## Hamlib settings non-blocking fix

- Hamlib model, capability and runtime-status probes are now dispatched from Wails event handlers into background goroutines. The event handlers return immediately and no longer block the UI thread/event dispatcher on `rigctl.exe`/DLL probing.
- Replaced the non-cancellable Hamlib operation mutex wait with a context-aware semaphore. Closing the settings page can cancel a queued probe without waiting for another Hamlib command to finish.
- Added an operation-generation guard so results from a settings session that has already been closed or invalidated are discarded.
- Added in-flight de-duplication for model list, capability and status requests to prevent repeated `rigctl.exe` launches when the settings UI emits duplicate requests.
- Reduced settings-page probe timeout to 3 seconds because model/capability/status probes must be fast and do not open a physical radio connection.
- Removed a duplicate legacy `hamlib:revert` event handler that could perform a second synchronous revert operation.
- Hamlib helper windows remain hidden, but this release explicitly fixes the underlying blocking behavior rather than merely suppressing console windows.
- Hamlib update/revert actions are also dispatched asynchronously so network/file operations do not block the GUI event dispatcher.

## Validation

- `gofmt` applied to modified Go sources.
- Core non-Wails Go packages can be tested in the current offline environment. Full `internal/app` validation requires the project's existing Wails module cache or a working Go module proxy.
