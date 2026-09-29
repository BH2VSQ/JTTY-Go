# JTTY-Go v5.36-fixed15

## Windows Hamlib console-window fix

- All Hamlib `rigctl`, `rigctld` and `taskkill` helper processes now use Windows `HideWindow` plus `CREATE_NO_WINDOW`.
- Long-running bundled `rigctld.exe` is started without inheriting stdout/stderr from the GUI process.
- This prevents Command Prompt windows from flashing when opening the radio configuration page, probing Hamlib capabilities, testing CAT/PTT, or starting the bundled `rigctld` backend.

## Hamlib probe resource control

- Hamlib model and capability probes are serialized so multiple settings-page requests cannot launch a burst of helper processes at the same time.
- Model lists, capability results and Hamlib runtime status are cached for the lifetime of the application and reused when reopening the configuration page.
- Duplicate capability requests for the same selected model are suppressed in the frontend while one request is already pending.
- Closing the settings dialog cancels in-flight Hamlib probes; switching away from the radio settings also stops the test/probe cleanup path.
- Probe commands have a bounded timeout so a broken/missing Hamlib executable cannot leave the settings workflow blocked indefinitely.
- Updating or reverting Hamlib invalidates the runtime/model/capability caches so the next read reflects the new DLL/runtime.

## UI/layout

- No changes to the existing radio configuration page layout.
- Existing WSJT-X-style capability-driven enable/disable behavior remains in place.
