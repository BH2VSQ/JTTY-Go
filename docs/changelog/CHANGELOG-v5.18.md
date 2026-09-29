# JTTY-Go v5.18

## UI / operation area

- TX audio output level is now a horizontal control directly below the RF frequency display.
- Removed the obsolete `JTTY —` label under the RF frequency display.
- Operation panel is fixed at 270 px height.
- Removed the splitter between the decode area and operation area.
- Only the waterfall/decode splitter remains user-adjustable.
- The persisted operation height is normalized to 270 px for backward-compatible config loading.

## Decode log files

Data directory: `%LOCALAPPDATA%\\JTTY-Go` on Windows.

- `ALL.txt`: all accepted/new JTTY decodes.
- Optional partitioning: single file, yearly `ALL-YYYY.txt`, or monthly `ALL-YYYY-MM.txt`.
- `JTTY.adi`: QSO log, always in the data directory.

Decode line format:

`YYMMDD_HHMMSS    RFMHz Rx JTTY     SNR  DT DF MESSAGE`

Example:

`260927_044527    50.316 Rx JTTY     0  0.0 1644 DE BG5JSU TNX SAT OM HPE CU AGN 73`

## File menu

- Delete ALL.txt and all year/month split decode log files.
- Delete `JTTY.adi`.
- Open the data/log directory.
- Exit JTTY-Go.
