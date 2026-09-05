#!/usr/bin/env bash
set -euo pipefail

exe="${1:?usage: check-windows-manifest.sh path/to/FTPServerGUI.exe}"

if ! strings "${exe}" | grep -Fq 'Microsoft.Windows.Common-Controls'; then
  echo "FAIL: Common Controls v6 manifest is missing from ${exe}" >&2
  exit 1
fi

if ! strings "${exe}" | grep -Fq 'PerMonitorV2'; then
  echo "FAIL: DPI awareness is missing from ${exe}" >&2
  exit 1
fi

echo "PASS: embedded Windows manifest found in ${exe}"
