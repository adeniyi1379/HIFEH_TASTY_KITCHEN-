//go:build windows

// Hifeh Tasty Kitchen — Uninstaller
// Registered in Add/Remove Programs by the installer.
// Removes app files, shortcuts, firewall rule, and registry entry.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procMessageBox   = user32.NewProc("MessageBoxW")
	procRegDeleteKey = advapi32.NewProc("RegDeleteKeyW")
)

const (
	HKEY_CURRENT_USER = 0x80000001
	MB_YESNO          = 0x00000004
	MB_YESNOCANCEL    = 0x00000003
	MB_ICONQUESTION   = 0x00000020
	MB_ICONINFO       = 0x00000040
	MB_ICONWARNING    = 0x00000030
	IDYES             = 6
	IDNO              = 7
	IDCANCEL          = 2
)

func utf16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func msgBox(title, text string, flags uintptr) int {
	ret, _, _ := procMessageBox.Call(0,
		uintptr(unsafe.Pointer(utf16(text))),
		uintptr(unsafe.Pointer(utf16(title))),
		flags,
	)
	return int(ret)
}

func main() {
	appName := "Hifeh Tasty Kitchen"
	installDir := filepath.Dir(os.Args[0])

	// Confirm uninstall
	ret := msgBox(appName+" — Uninstall",
		fmt.Sprintf("Are you sure you want to uninstall %s?\n\nThis will remove the app from:\n%s", appName, installDir),
		MB_YESNO|MB_ICONQUESTION,
	)
	if ret != IDYES {
		return
	}

	// Ask about data
	deleteData := false
	ret = msgBox("Delete Data?",
		"Do you also want to delete your sales data (pos.db)?\n\n"+
			"YES = Delete everything (cannot be undone!)\n"+
			"NO = Keep data (you can restore it later)\n"+
			"CANCEL = Abort uninstall",
		MB_YESNOCANCEL|MB_ICONWARNING,
	)
	if ret == IDCANCEL {
		return
	}
	deleteData = (ret == IDYES)

	// Kill running instance
	exec.Command("taskkill", "/F", "/IM", "posd.exe").Run()

	// Remove desktop shortcut
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", appName+".lnk")
	os.Remove(desktop)

	// Remove Start Menu shortcuts
	smDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", appName)
	os.RemoveAll(smDir)

	// Remove firewall rule
	exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name="+appName).Run()

	// Remove registry entry (Add/Remove Programs)
	keyPath := utf16(`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\HifehTastyKitchen`)
	procRegDeleteKey.Call(HKEY_CURRENT_USER, uintptr(unsafe.Pointer(keyPath)))

	// Remove app files
	os.Remove(filepath.Join(installDir, "posd.exe"))
	os.Remove(filepath.Join(installDir, "README.txt"))
	os.Remove(filepath.Join(installDir, "app.ico"))

	if deleteData {
		os.Remove(filepath.Join(installDir, "pos.db"))
		os.Remove(filepath.Join(installDir, "pos.db-shm"))
		os.Remove(filepath.Join(installDir, "pos.db-wal"))
	}

	msgBox("Uninstall Complete",
		appName+" has been removed from your computer.\n\n"+
			"The install folder will be cleaned up automatically.",
		MB_ICONINFO,
	)

	// Self-delete: spawn cmd to delete uninstall.exe and the folder after a delay
	self := os.Args[0]
	cmd := fmt.Sprintf(`/c timeout /t 3 /nobreak >nul && del /f /q "%s" && rd "%s" 2>nul`, self, installDir)
	c := exec.Command("cmd", cmd)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	c.Start()
}
