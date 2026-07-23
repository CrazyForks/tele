//go:build windows

package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// newOSClipboardImageReader returns the PowerShell-backed reader on Windows.
func newOSClipboardImageReader() clipboardImageReader { return windowsClipboardImageReader{} }

// windowsClipboardImageReader reads a clipboard image by shelling out to Windows
// PowerShell, which ships with the OS and runs STA — required by
// System.Windows.Forms.Clipboard. The script saves the image as PNG to a temp
// file we read back; stdout carries a status so "no image" (silent text paste)
// is distinguished from a real failure (toast).
type windowsClipboardImageReader struct{}

const (
	clipboardPSOK      = "OK"
	clipboardPSNoImage = "NOIMAGE"
)

// clipboardPowershellScript builds the one-liner that saves a clipboard image to
// path as PNG. Single quotes in the path are doubled so it stays inside the
// PowerShell single-quoted literal.
func clipboardPowershellScript(path string) string {
	p := strings.ReplaceAll(path, "'", "''")
	return "Add-Type -AssemblyName System.Windows.Forms,System.Drawing; " +
		"if ([System.Windows.Forms.Clipboard]::ContainsImage()) { " +
		"$img = [System.Windows.Forms.Clipboard]::GetImage(); " +
		"$img.Save('" + p + "', [System.Drawing.Imaging.ImageFormat]::Png); " +
		"Write-Output '" + clipboardPSOK + "' } else { Write-Output '" + clipboardPSNoImage + "' }"
}

// powershellClipboardCmd builds the powershell invocation for the save script.
func powershellClipboardCmd(path string) *exec.Cmd {
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA",
		"-Command", clipboardPowershellScript(path))
}

// interpretWindowsClipboardResult decides the reader outcome from the script's
// stdout, the command error, and the temp-file read. Missing PowerShell or a
// "no image" status degrades silently; a script failure reports an error so the
// caller can toast.
func interpretWindowsClipboardResult(stdout string, runErr error, data []byte, readErr error) ([]byte, string, error) {
	s := strings.TrimSpace(stdout)
	if runErr != nil {
		if errors.Is(runErr, exec.ErrNotFound) {
			return nil, "", nil // PowerShell unavailable -> fall back to text
		}
		return nil, "", fmt.Errorf("powershell clipboard: %w", runErr)
	}
	if strings.Contains(s, clipboardPSNoImage) {
		return nil, "", nil // no image on the clipboard
	}
	if readErr != nil {
		return nil, "", readErr
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("clipboard image was empty")
	}
	return data, ".png", nil
}

func (windowsClipboardImageReader) ReadImage() ([]byte, string, error) {
	f, err := os.CreateTemp("", "tele-clip-*.png")
	if err != nil {
		return nil, "", err
	}
	path := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(path) }()

	out, runErr := powershellClipboardCmd(path).Output()
	data, readErr := os.ReadFile(path)
	return interpretWindowsClipboardResult(string(out), runErr, data, readErr)
}
