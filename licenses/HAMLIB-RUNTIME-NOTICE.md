# Bundled Windows Hamlib runtime

The existing upstream JTTY-Go binaries are pinned as one Windows amd64 runtime:
Hamlib 4.7.3~rc, 2026-09-22T20:59:11Z, source commit d034b2365.

Source: https://github.com/Hamlib/Hamlib/tree/d034b2365
Source archive: https://github.com/Hamlib/Hamlib/archive/d034b2365.tar.gz
Per-file SHA-256 values are in hamlib/runtime-manifest.json (bin/ in the source tree).

- rigctl/rigctld: Hamlib GPL; see HAMLIB-GPL-2.0.txt.
- libhamlib and libusb: LGPL; see HAMLIB-LGPL-2.1.txt.
- libgcc_s_seh and libwinpthread: GCC/MinGW runtime components. See GCC-RUNTIME-EXCEPTION.txt and the accompanying GPL-3.0 text in WSJT-X-GPL-3.0.txt; MinGW notices are in MINGW-RUNTIME-NOTICES.txt.

The program verifies the whole runtime before launching local Hamlib. NET rigctl
uses an externally managed server and does not replace any of its files.
