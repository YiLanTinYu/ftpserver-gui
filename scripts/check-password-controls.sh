#!/bin/sh
set -eu

source_file="${1:-cmd/ftpserver-gui/main_windows.go}"

grep -Fq 'Text: "显示密码"' "${source_file}"
grep -Fq 'Text: "复制密码"' "${source_file}"
grep -Fq 'walk.Clipboard().SetText(password)' "${source_file}"
grep -Fq 'a.password.SetPasswordMode(!a.showPassword.Checked())' "${source_file}"
echo "PASS: password can be generated, revealed and copied"
