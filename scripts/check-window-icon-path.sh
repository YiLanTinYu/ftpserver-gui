#!/bin/sh
set -eu

source_file="${1:-cmd/ftpserver-gui/main_windows.go}"

grep -Fq 'Icon:     a.appIcon' "${source_file}"
grep -Fq 'a.appIcon, err = embeddedLogoIcon()' "${source_file}"
echo "PASS: main window explicitly uses the embedded application logo"
