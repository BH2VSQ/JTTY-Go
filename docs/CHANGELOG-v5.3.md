# JTTY-Go Alpha v5.3

## Windows audio compile fix

Fixed the COM initialization helper for `github.com/go-ole/go-ole v1.2.6`.
That version does not export `ole.S_FALSE`; `CoInitializeEx` reports HRESULT
`S_FALSE (0x00000001)` through `*ole.OleError`, so the helper now checks the
HRESULT value directly.

No JTTY DSP/decoder behavior was changed in this release.
