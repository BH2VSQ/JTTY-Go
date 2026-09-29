# JTTY-Go Alpha v5.17

## Main window layout

- Removed the waterfall title bar.
- Compressed the frequency ruler to 24 px and reduced the default waterfall height to 210 px.
- Removed the legacy mode-switch buttons and the five lookup/management buttons from the operation area.
- RX/TX audio-frequency controls and tolerance now share a single row.
- RX and TX frequency controls are native number inputs with spinner controls.
- Added two transfer buttons: `←` performs `TX = RX`; `→` performs `RX = TX`.
- Tolerance is a dedicated spinner input, clamped to the JTTY implementation range used by this project.
- The main RF frequency display is now a compact editable field. Enter/change commits the radio frequency through the backend.
- The lower-left vertical level control is explicitly a TX output-audio level control, not RF power.
- Added a visible TX audio level percentage next to the level control.
- Reworked the operation area's center layout to avoid bottom clipping.

## User-resizable layout

- Added a draggable splitter between the waterfall and decode area.
- Added a draggable splitter between the decode area and operation area.
- Splitter movement is clamped by configured minimum/maximum region heights.
- Layout values are automatically persisted to the user settings after dragging.
- Added `Window layout` controls and reset-to-default support to the General settings page.
- Default layout: waterfall 210 px, operation 295 px.
- Minimum layout constraints: waterfall 150 px, decode 190 px, operation 250 px.

## State synchronization

- `settings:saved` no longer overwrites live state with stale settings-draft data.
- TX audio level changes update the main slider and are debounced before persistence.
- Layout is reapplied on restored settings and on window resize.

## JTTY tolerance

- The backend JTTY decoder exposes a synchronized frequency-tolerance setting.
- The UI keeps the JTTY tolerance control connected to the same backend value used by the decoder.

## Verification

- TypeScript syntax/type-check passes with the local `tsc` compiler.
- Core packages that do not require unavailable Wails module checks were tested successfully in the build environment.
