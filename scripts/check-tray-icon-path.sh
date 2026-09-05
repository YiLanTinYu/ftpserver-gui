#!/usr/bin/env bash
set -euo pipefail

source_file="cmd/ftpserver-gui/main_windows.go"

if grep -Fq 'NewIconFromResourceId' "${source_file}"; then
  echo "FAIL: tray icon still uses the LoadIconWithScaleDown resource path" >&2
  exit 1
fi

grep -Fq 'NewIconFromImageForDPI' "${source_file}"
grep -Fq '//go:embed ftpserver-logo.png' "${source_file}"
echo "PASS: tray icon uses embedded PNG and bypasses LoadIconWithScaleDown"
