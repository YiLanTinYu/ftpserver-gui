#!/usr/bin/env bash
set -euo pipefail

version="${1:-0.1.0}"
output_dir="dist/FTP绿色版服务端_v${version}_Windows_x64"

mkdir -p "${output_dir}/data" "${output_dir}/logs"
# rsrc_windows_amd64.syso is kept in source control so normal offline builds do
# not require the resource compiler. Regenerate it whenever the manifest changes:
# go run github.com/akavel/rsrc@v0.10.2 -arch amd64 \
#   -manifest cmd/ftpserver-gui/ftpserver-gui.manifest \
#   -ico cmd/ftpserver-gui/ftpserver-gui.ico \
#   -o cmd/ftpserver-gui/rsrc_windows_amd64.syso
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags localfs \
  -trimpath \
  -ldflags="-s -w -H=windowsgui" \
  -o "${output_dir}/FTPServerGUI.exe" \
  ./cmd/ftpserver-gui

scripts/check-windows-manifest.sh "${output_dir}/FTPServerGUI.exe"
scripts/check-tray-icon-path.sh
scripts/check-window-icon-path.sh
scripts/check-windows-startup-path.sh
scripts/check-password-controls.sh

cp LICENSE "${output_dir}/LICENSE.txt"
cp packaging/README_zh-CN.txt "${output_dir}/使用说明.txt"

(
  cd dist
  zip -qr "FTP绿色版服务端_v${version}_Windows_x64.zip" "FTP绿色版服务端_v${version}_Windows_x64"
)
