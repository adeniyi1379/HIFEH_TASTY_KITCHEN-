//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed embed/posd.exe
var posExe []byte

// --- Windows API (MessageBox) ---

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	mbOK             = 0x00000000
	mbOKCancel       = 0x00000001
	mbYesNo          = 0x00000004
	mbIconInfo       = 0x00000040
	mbIconQuestion   = 0x00000020
	mbIconError      = 0x00000010
	mbIconExclamation = 0x00000030
	idOK             = 1
	idCancel         = 2
	idYes            = 6
	idNo             = 7
)

func msgBox(title, text string, flags uintptr) int {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	ret, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		flags,
	)
	return int(ret)
}

// --- PowerShell helpers ---

func runPS(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func pickFolder() string {
	script := `
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.FolderBrowserDialog
$f.Description      = "Choose where to install Hifeh Tasty Kitchen"
$f.RootFolder       = "MyComputer"
$f.ShowNewFolderButton = $true
if ($f.ShowDialog() -eq "OK") { $f.SelectedPath } else { "" }
`
	out, err := runPS(script)
	if err != nil {
		return ""
	}
	return out
}

func createDesktopShortcut(exePath string) error {
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	lnk := filepath.Join(desktop, "Hifeh Tasty Kitchen.lnk")
	workDir := filepath.Dir(exePath)

	script := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s  = $ws.CreateShortcut('%s')
$s.TargetPath       = '%s'
$s.WorkingDirectory  = '%s'
$s.Description       = 'Hifeh Tasty Kitchen POS System'
$s.Save()
`, lnk, exePath, workDir)

	_, err := runPS(script)
	return err
}

func createStartMenuShortcut(exePath string) {
	startMenu := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Hifeh Tasty Kitchen")
	os.MkdirAll(startMenu, 0755)
	lnk := filepath.Join(startMenu, "Hifeh Tasty Kitchen.lnk")
	workDir := filepath.Dir(exePath)

	script := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s  = $ws.CreateShortcut('%s')
$s.TargetPath       = '%s'
$s.WorkingDirectory  = '%s'
$s.Description       = 'Hifeh Tasty Kitchen POS System'
$s.Save()
`, lnk, exePath, workDir)

	runPS(script)
}

func addFirewallRule(exePath string) {
	script := fmt.Sprintf(
		`netsh advfirewall firewall add rule name="Hifeh Tasty Kitchen" dir=in action=allow program="%s" enable=yes profile=private`,
		exePath,
	)
	exec.Command("cmd", "/c", script).Run()
}

// --- Main ---

func main() {
	appName := "Hifeh Tasty Kitchen"

	// ── Welcome ──
	ret := msgBox(
		appName+" — Setup",
		"Welcome to Hifeh Tasty Kitchen!\n\n"+
			"This will install the POS system on your computer.\n"+
			"It works completely offline — no internet needed.\n\n"+
			"Click OK to choose where to install.",
		mbOKCancel|mbIconInfo,
	)
	if ret != idOK {
		return
	}

	// ── Pick folder ──
	folder := pickFolder()
	if folder == "" {
		return // user cancelled
	}

	// ── Confirm ──
	destExe := filepath.Join(folder, "posd.exe")
	ret = msgBox(
		"Confirm Installation",
		fmt.Sprintf("Install %s to:\n\n  %s\n\nA desktop shortcut will be created.\n\nContinue?", appName, folder),
		mbYesNo|mbIconQuestion,
	)
	if ret != idYes {
		return
	}

	// ── Check if already running ──
	exec.Command("taskkill", "/F", "/IM", "posd.exe").Run()

	// ── Create folder ──
	if err := os.MkdirAll(folder, 0755); err != nil {
		msgBox("Error", "Could not create folder:\n\n"+err.Error(), mbOK|mbIconError)
		return
	}

	// ── Write posd.exe ──
	if err := os.WriteFile(destExe, posExe, 0755); err != nil {
		msgBox("Error", "Could not write posd.exe:\n\n"+err.Error(), mbOK|mbIconError)
		return
	}

	// ── Copy README ──
	readme := `# Hifeh Tasty Kitchen — POS System

Your restaurant's point of sale — take orders, print receipts, and track sales.
Works offline, no internet needed.

## How to Start
1. Double-click the "Hifeh Tasty Kitchen" shortcut on your Desktop
2. Open Google Chrome and go to: http://localhost:8080
3. Enter your PIN to log in

## Default PINs
- Owner:          123456
- Manager (Ada):  234567
- Cashier (Amaka): 345678
- Cashier (Tunde): 456789

## Important
- Do NOT close the black terminal window while using the system
- Do NOT delete the pos.db file — it contains all your data
- If Windows asks "Do you want to allow this app?" — click Yes
`
	os.WriteFile(filepath.Join(folder, "README.txt"), []byte(readme), 0644)

	// ── Create shortcuts ──
	createDesktopShortcut(destExe)
	createStartMenuShortcut(destExe)

	// ── Firewall rule ──
	addFirewallRule(destExe)

	// ── Success ──
	msgBox(
		"Installation Complete!",
		fmt.Sprintf(
			"%s has been installed successfully!\n\n"+
				"📂 Location: %s\n"+
				"🖥️ Desktop shortcut created\n"+
				"📋 Start Menu shortcut created\n"+
				"🔥 Firewall rule added\n\n"+
				"Default Owner PIN: 123456",
			appName, folder,
		),
		mbOK|mbIconInfo,
	)

	// ── Launch? ──
	ret = msgBox(
		"Launch Now?",
		"Would you like to start "+appName+" now?",
		mbYesNo|mbIconQuestion,
	)
	if ret == idYes {
		cmd := exec.Command(destExe)
		cmd.Dir = folder
		cmd.Start()

		// Open browser after a short delay
		exec.Command("cmd", "/c", "timeout /t 3 /nobreak >nul && start http://localhost:8080").Start()
	}
}
