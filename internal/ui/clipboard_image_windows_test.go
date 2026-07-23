//go:build windows

package ui

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClipboardPowershellScript(t *testing.T) {
	s := clipboardPowershellScript(`C:\Temp\clip.png`)
	assert.Contains(t, s, "System.Windows.Forms.Clipboard]::ContainsImage()")
	assert.Contains(t, s, "GetImage()")
	assert.Contains(t, s, "ImageFormat]::Png")
	assert.Contains(t, s, `'C:\Temp\clip.png'`)
	assert.Contains(t, s, "'"+clipboardPSOK+"'")
	assert.Contains(t, s, "'"+clipboardPSNoImage+"'")
}

func TestClipboardPowershellScript_EscapesSingleQuote(t *testing.T) {
	// A single quote in the path is doubled to stay inside the PS literal.
	s := clipboardPowershellScript(`C:\a'b\clip.png`)
	assert.Contains(t, s, `'C:\a''b\clip.png'`)
}

func TestPowershellClipboardCmd_Args(t *testing.T) {
	cmd := powershellClipboardCmd(`C:\Temp\clip.png`)
	assert.Equal(t, "powershell", cmd.Args[0])
	assert.Equal(t, []string{"-NoProfile", "-NonInteractive", "-STA", "-Command"}, cmd.Args[1:5])
	assert.Equal(t, clipboardPowershellScript(`C:\Temp\clip.png`), cmd.Args[5])
}

func TestInterpretWindowsClipboardResult_Success(t *testing.T) {
	data, ext, err := interpretWindowsClipboardResult("OK\r\n", nil, []byte("PNGDATA"), nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("PNGDATA"), data)
	assert.Equal(t, ".png", ext)
}

func TestInterpretWindowsClipboardResult_NoImage_Silent(t *testing.T) {
	data, _, err := interpretWindowsClipboardResult("NOIMAGE\r\n", nil, nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestInterpretWindowsClipboardResult_PowershellMissing_Silent(t *testing.T) {
	missing := &exec.Error{Name: "powershell", Err: exec.ErrNotFound}
	data, _, err := interpretWindowsClipboardResult("", missing, nil, nil)
	assert.NoError(t, err, "a missing PowerShell degrades to text, not an error")
	assert.Nil(t, data)
}

func TestInterpretWindowsClipboardResult_ScriptFailure_ReturnsError(t *testing.T) {
	_, _, err := interpretWindowsClipboardResult("", errors.New("exit status 1"), nil, nil)
	assert.Error(t, err, "a script failure must report an error (toast)")
}

func TestInterpretWindowsClipboardResult_EmptyImage_ReturnsError(t *testing.T) {
	_, _, err := interpretWindowsClipboardResult("OK", nil, nil, nil)
	assert.Error(t, err)
}
