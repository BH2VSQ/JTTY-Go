# JTTY-Go Alpha v5.6

## Windows Wails dev fix

Wails v2.15.0 distinguishes the development frontend build command from the
long-running frontend watcher. The project previously configured
`frontend:dev` as `npm run dev`, which caused `wails dev` to stay forever at
`Compiling frontend:` because Vite's development server never exits.

The configuration now uses:

- `frontend:dev:build = npm run build`
- `frontend:dev:install = npm install`
- `frontend:dev:watcher = npm run dev`
- `frontend:dev:serverUrl = auto`

The legacy `frontend:dev` key is removed.

## Frontend module mode

`frontend/package.json` now declares `type: module`, matching the ESM Vite
configuration and removing the Node CommonJS/ESM experimental warning from the
empty `vite.config.ts`.
