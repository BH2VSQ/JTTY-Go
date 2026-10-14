# WSJT-X JTTY reference sources

This directory contains selected JTTY-related source files copied from the
original user-provided `wsjtx-3.2.0-rc1-src.tar.gz` archive, refreshed against
WSJT-X commit `9d059511f77e2d1be746ebfeaa7f88e77c1babb7` (release/3.3, 2026-10-13).
`SOURCE_ARCHIVE_SHA256.txt` records the original archive; `UPSTREAM_COMMIT.txt`
records the current algorithm baseline. They are retained as an
algorithmic and interoperability reference for JTTY-Go.

The reference files remain governed by the WSJT-X project license and source
notices. JTTY-Go does not compile these Fortran/C++ files into the application;
the runtime implementation in `internal/jtty` is a Go reimplementation.

Primary files used during the port include:

- `lib/jtty/jtty_decode.f90`
- `lib/jtty/jtty_mdecode.f90`
- `lib/jtty/rjtty_sub.f90`
- `lib/jtty/jtty_peakup.f90`
- `lib/jtty/jtty_payload_correlators.f90`
- `lib/jtty/jtty_tbcc_decoder.f90`
- `lib/jtty/jtty_tbcc_list_decoder.f90`
- `lib/jtty/jtty_source_codec.f90`
- `lib/jtty/jtty_mod.f90`
- `lib/jtty/jtty_fec_mod.f90`
- `lib/jtty/gen_jttywave.f90`
- `lib/jtty/gen_syncwave.f90`
- `lib/jtty/subtract_jtty.f90`
- `widgets/JttyMessages.hpp`
- `widgets/mainwindow_jtty.cpp`
