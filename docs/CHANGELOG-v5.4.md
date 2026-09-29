# JTTY-Go v5.4

## Fixed

- Corrected the Wails v2 asset server configuration in `main.go`.
- Replaced the non-existent `options.AssetServer` type with `pkg/options/assetserver.Options`.
- Frontend assets continue to be supplied from the embedded `frontend/dist` filesystem.

This matches the Wails v2 API where `options.App.AssetServer` expects `*assetserver.Options` and the embedded filesystem is assigned to `assetserver.Options.Assets`.
