#!/bin/sh
set -eu

source_file="${1:-cmd/ftpserver-gui/main_windows.go}"

grep -Fq 'registry.CURRENT_USER' "${source_file}"
grep -Fq 'Software\Microsoft\Windows\CurrentVersion\Run' "${source_file}"
grep -Fq 'key.SetStringValue(startupValueName, startupCommand(exePath))' "${source_file}"
grep -Fq 'key.DeleteValue(startupValueName)' "${source_file}"
echo "PASS: startup checkbox writes and removes the current-user Run entry"
