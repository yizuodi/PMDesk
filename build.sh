#!/usr/bin/env bash
# Build PM Jira. Produces:
#   pmjira         — current platform (console)
#   pmjira.exe     — Windows console build (shows logs in a terminal window)
#   pmjira-gui.exe — Windows no-console build (-H windowsgui): double-click runs
#                    silently in the background; stop it via the 设置 → 停止服务
#                    button. Logs go to pmjira.log next to the database.
set -euo pipefail
cd "$(dirname "$0")"

LDFLAGS="-s -w"
GUI_LDFLAGS="-s -w -H windowsgui -X main.guiBuild=true"

echo "→ building for $(go env GOOS)/$(go env GOARCH) …"
CGO_ENABLED=0 go build -trimpath -ldflags="$LDFLAGS" -o pmjira .

echo "→ cross-compiling windows/amd64 (console) …"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$LDFLAGS" -o pmjira.exe .

echo "→ cross-compiling windows/amd64 (no-console / windowsgui) …"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$GUI_LDFLAGS" -o pmjira-gui.exe .

echo "✓ done"
ls -lh pmjira pmjira.exe pmjira-gui.exe
