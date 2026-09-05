package main

import "testing"

func TestStartupCommandQuotesExecutablePath(t *testing.T) {
	got := startupCommand(`C:\Program Files\FTP绿色版服务端\FTPServerGUI.exe`)
	want := `"C:\Program Files\FTP绿色版服务端\FTPServerGUI.exe"`
	if got != want {
		t.Fatalf("startupCommand() = %q, want %q", got, want)
	}
}
