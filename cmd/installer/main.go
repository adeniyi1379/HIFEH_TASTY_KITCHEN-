//go:build windows

// Hifeh Tasty Kitchen — Native Windows GUI Installer
// Cross-compiled from Linux using raw Win32 API via syscall.
// No CGO, no external dependencies.

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
var embeddedPOS []byte

//go:embed embed/uninstall.exe
var embeddedUninstaller []byte

//go:embed embed/app.ico
var embeddedIcon []byte

// ══════════════════════════════════════════════════════════════════════
// Windows API bindings
// ══════════════════════════════════════════════════════════════════════

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procGetModuleHandle  = kernel32.NewProc("GetModuleHandleW")
	procCreateActCtx     = kernel32.NewProc("CreateActCtxW")
	procActivateActCtx   = kernel32.NewProc("ActivateActCtx")
	procRegisterClassEx  = user32.NewProc("RegisterClassExW")
	procCreateWindowEx   = user32.NewProc("CreateWindowExW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procGetMessage       = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessageW")
	procDefWindowProc    = user32.NewProc("DefWindowProcW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procSendMessage      = user32.NewProc("SendMessageW")
	procGetWindowText    = user32.NewProc("GetWindowTextW")
	procSetWindowText    = user32.NewProc("SetWindowTextW")
	procEnableWindow     = user32.NewProc("EnableWindow")
	procMessageBox       = user32.NewProc("MessageBoxW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procLoadCursor       = user32.NewProc("LoadCursorW")
	procLoadIcon         = user32.NewProc("LoadIconW")
	procGetDC            = user32.NewProc("GetDC")
	procReleaseDC        = user32.NewProc("ReleaseDC")
	procGetStockObject   = gdi32.NewProc("GetStockObject")
	procCreateFont       = gdi32.NewProc("CreateFontW")
	procGetDeviceCaps    = gdi32.NewProc("GetDeviceCaps")
	procRegCreateKeyEx   = advapi32.NewProc("RegCreateKeyExW")
	procRegSetValueEx    = advapi32.NewProc("RegSetValueExW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

// Window messages
const (
	WM_CREATE         = 0x0001
	WM_DESTROY        = 0x0002
	WM_CLOSE          = 0x0010
	WM_SETFONT        = 0x0030
	WM_COMMAND        = 0x0111
	WM_CTLCOLORSTATIC = 0x0138
)

// Window/control styles
const (
	WS_OVERLAPPED    = 0x00000000
	WS_CAPTION       = 0x00C00000
	WS_SYSMENU       = 0x00080000
	WS_MINIMIZEBOX   = 0x00020000
	WS_VISIBLE       = 0x10000000
	WS_CHILD         = 0x40000000
	WS_TABSTOP       = 0x00010000
	WS_GROUP         = 0x00020000
	WS_EX_CLIENTEDGE = 0x00000200
	BS_PUSHBUTTON    = 0x00000000
	BS_DEFPUSHBUTTON = 0x00000001
	BS_AUTOCHECKBOX  = 0x00000003
	BS_GROUPBOX      = 0x00000007
	SS_LEFT          = 0x00000000
	SS_CENTER        = 0x00000001
	ES_AUTOHSCROLL   = 0x00000080
	BM_GETCHECK      = 0x00F0
	BM_SETCHECK      = 0x00F1
	BST_CHECKED      = 1
	BST_UNCHECKED    = 0
)

// System metrics / colors
const (
	SM_CXSCREEN     = 0
	SM_CYSCREEN     = 1
	COLOR_BTNFACE   = 15
	WHITE_BRUSH     = 0
	LOGPIXELSY      = 90
	IDC_ARROW       = 32512
	IDI_APPLICATION = 32512
)

// Button notification
const BN_CLICKED = 0

// Registry
const (
	HKEY_CURRENT_USER = 0x80000001
	KEY_ALL_ACCESS    = 0xF003F
	REG_SZ            = 1
	REG_DWORD         = 4
)

// Control IDs
const (
	IDC_PATH_EDIT     = 1001
	IDC_BROWSE_BTN    = 1002
	IDC_CHK_DESKTOP   = 1003
	IDC_CHK_STARTMENU = 1004
	IDC_CHK_FIREWALL  = 1005
	IDC_CHK_LAUNCH    = 1006
	IDC_INSTALL_BTN   = 1007
	IDC_CANCEL_BTN    = 1008
	IDC_STATUS_LABEL  = 1009
)

// Win32 structs
type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      [2]int32
}

type ACTCTX struct {
	CbSize                 uint32
	DwFlags                uint32
	LpSource               *uint16
	WProcessorArchitecture uint16
	LangId                 uint16
	LpAssemblyDirectory    *uint16
	LpResourceName         uintptr
	LpApplicationName      *uint16
	HModule                uintptr
}

// ══════════════════════════════════════════════════════════════════════
// Global state
// ══════════════════════════════════════════════════════════════════════

var (
	hWndMain      uintptr
	hWndPathEdit  uintptr
	hWndStatus    uintptr
	hWndInstall   uintptr
	hWndCancel    uintptr
	hFont         uintptr
	hFontTitle    uintptr
	hFontSubtitle uintptr
	whiteBrush    uintptr
	hInstance     uintptr
	defaultPath   string
)

// ══════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════

func utf16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func msgBox(hwnd uintptr, title, text string, flags uintptr) int {
	ret, _, _ := procMessageBox.Call(hwnd,
		uintptr(unsafe.Pointer(utf16(text))),
		uintptr(unsafe.Pointer(utf16(title))),
		flags,
	)
	return int(ret)
}

func createFont(name string, height int32, weight int32) uintptr {
	h, _, _ := procCreateFont.Call(
		uintptr(height),                      // nHeight
		0,                                    // nWidth
		0,                                    // nEscapement
		0,                                    // nOrientation
		uintptr(weight),                      // fnWeight
		0,                                    // fdwItalic
		0,                                    // fdwUnderline
		0,                                    // fdwStrikeOut
		1,                                    // fdwCharSet (DEFAULT_CHARSET)
		0,                                    // fdwOutputPrecision
		0,                                    // fdwClipPrecision
		0,                                    // fdwQuality
		0,                                    // fdwPitchAndFamily
		uintptr(unsafe.Pointer(utf16(name))), // lpszFace
	)
	return h
}

func setFont(hwnd, font uintptr) {
	procSendMessage.Call(hwnd, WM_SETFONT, font, 1)
}

func setCheck(hwnd uintptr, checked bool) {
	v := uintptr(BST_UNCHECKED)
	if checked {
		v = BST_CHECKED
	}
	procSendMessage.Call(hwnd, BM_SETCHECK, v, 0)
}

func isChecked(hwnd uintptr) bool {
	ret, _, _ := procSendMessage.Call(hwnd, BM_GETCHECK, 0, 0)
	return ret == BST_CHECKED
}

func getWindowText(hwnd uintptr) string {
	buf := make([]uint16, 512)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 512)
	return syscall.UTF16ToString(buf)
}

func setStatus(text string) {
	procSetWindowText.Call(hWndStatus, uintptr(unsafe.Pointer(utf16(text))))
}

func createControl(class, text string, style uintptr, x, y, w, h int32, id uintptr) uintptr {
	hwnd, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(utf16(class))),
		uintptr(unsafe.Pointer(utf16(text))),
		WS_CHILD|WS_VISIBLE|style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hWndMain, id, hInstance, 0,
	)
	return hwnd
}

func createControlEx(exStyle uintptr, class, text string, style uintptr, x, y, w, h int32, id uintptr) uintptr {
	hwnd, _, _ := procCreateWindowEx.Call(
		exStyle,
		uintptr(unsafe.Pointer(utf16(class))),
		uintptr(unsafe.Pointer(utf16(text))),
		WS_CHILD|WS_VISIBLE|style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hWndMain, id, hInstance, 0,
	)
	return hwnd
}

// CREATE_NO_WINDOW keeps console children from flashing a black window.
const createNoWindow = 0x08000000

func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// openBrowserLater opens the till page once posd has had a moment to start.
func openBrowserLater() {
	cmd := exec.Command("cmd", "/c", "timeout /t 3 /nobreak >nul && start http://localhost:8080")
	hideConsole(cmd)
	cmd.Start()
}

// ══════════════════════════════════════════════════════════════════════
// Visual styles (themed controls)
// ══════════════════════════════════════════════════════════════════════

func enableVisualStyles() {
	manifest := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls"
        version="6.0.0.0" processorArchitecture="*"
        publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
</assembly>`

	tmpDir := os.TempDir()
	mPath := filepath.Join(tmpDir, "hifeh_setup.manifest")
	if err := os.WriteFile(mPath, []byte(manifest), 0644); err != nil {
		return
	}
	defer os.Remove(mPath)

	ctx := ACTCTX{
		CbSize:   uint32(unsafe.Sizeof(ACTCTX{})),
		DwFlags:  0,
		LpSource: utf16(mPath),
	}
	hCtx, _, _ := procCreateActCtx.Call(uintptr(unsafe.Pointer(&ctx)))
	if hCtx == 0 || hCtx == ^uintptr(0) {
		return
	}
	var cookie uintptr
	procActivateActCtx.Call(hCtx, uintptr(unsafe.Pointer(&cookie)))
}

// ══════════════════════════════════════════════════════════════════════
// Window procedure
// ══════════════════════════════════════════════════════════════════════

var (
	hWndChkDesktop   uintptr
	hWndChkStartMenu uintptr
	hWndChkFirewall  uintptr
	hWndChkLaunch    uintptr
)

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		hWndMain = hwnd

		// ── DPI-aware font sizes ──
		hdc, _, _ := procGetDC.Call(0)
		dpi, _, _ := procGetDeviceCaps.Call(hdc, LOGPIXELSY)
		procReleaseDC.Call(0, hdc)
		if dpi == 0 {
			dpi = 96
		}

		titleSize := -int32(18 * dpi / 72)
		subtitleSize := -int32(10 * dpi / 72)
		bodySize := -int32(9 * dpi / 72)

		hFontTitle = createFont("Segoe UI", titleSize, 700) // Bold
		hFontSubtitle = createFont("Segoe UI", subtitleSize, 400)
		hFont = createFont("Segoe UI", bodySize, 400)

		whiteBrush, _, _ = procGetStockObject.Call(WHITE_BRUSH)

		// ── Header ──
		h := createControl("STATIC", "\U0001F37D  Hifeh Tasty Kitchen", SS_CENTER, 0, 15, 520, 30, 0)
		setFont(h, hFontTitle)

		h = createControl("STATIC", "Point of Sale — Setup", SS_CENTER, 0, 48, 520, 20, 0)
		setFont(h, hFontSubtitle)

		// ── Install Location group ──
		h = createControl("BUTTON", " Install Location", BS_GROUPBOX, 20, 85, 480, 65, 0)
		setFont(h, hFont)

		hWndPathEdit = createControlEx(WS_EX_CLIENTEDGE, "EDIT", defaultPath,
			ES_AUTOHSCROLL|WS_TABSTOP, 35, 108, 355, 24, IDC_PATH_EDIT)
		setFont(hWndPathEdit, hFont)

		h = createControl("BUTTON", "Browse...", BS_PUSHBUTTON|WS_TABSTOP, 400, 107, 85, 26, IDC_BROWSE_BTN)
		setFont(h, hFont)

		// ── Options group ──
		h = createControl("BUTTON", " Options", BS_GROUPBOX, 20, 165, 480, 140, 0)
		setFont(h, hFont)

		hWndChkDesktop = createControl("BUTTON", "Create Desktop shortcut", BS_AUTOCHECKBOX|WS_TABSTOP, 40, 188, 440, 22, IDC_CHK_DESKTOP)
		setFont(hWndChkDesktop, hFont)
		setCheck(hWndChkDesktop, true)

		hWndChkStartMenu = createControl("BUTTON", "Create Start Menu shortcut", BS_AUTOCHECKBOX|WS_TABSTOP, 40, 213, 440, 22, IDC_CHK_STARTMENU)
		setFont(hWndChkStartMenu, hFont)
		setCheck(hWndChkStartMenu, true)

		hWndChkFirewall = createControl("BUTTON", "Allow LAN access (Windows Firewall rule)", BS_AUTOCHECKBOX|WS_TABSTOP, 40, 238, 440, 22, IDC_CHK_FIREWALL)
		setFont(hWndChkFirewall, hFont)
		setCheck(hWndChkFirewall, true)

		hWndChkLaunch = createControl("BUTTON", "Launch app after install", BS_AUTOCHECKBOX|WS_TABSTOP, 40, 263, 440, 22, IDC_CHK_LAUNCH)
		setFont(hWndChkLaunch, hFont)
		setCheck(hWndChkLaunch, true)

		// ── Buttons ──
		hWndInstall = createControl("BUTTON", "Install", BS_DEFPUSHBUTTON|WS_TABSTOP, 280, 325, 110, 35, IDC_INSTALL_BTN)
		setFont(hWndInstall, hFont)

		hWndCancel = createControl("BUTTON", "Cancel", BS_PUSHBUTTON|WS_TABSTOP, 400, 325, 85, 35, IDC_CANCEL_BTN)
		setFont(hWndCancel, hFont)

		// ── Status label ──
		hWndStatus = createControl("STATIC", "Ready to install.", SS_LEFT, 25, 375, 470, 20, IDC_STATUS_LABEL)
		setFont(hWndStatus, hFont)

		return 0

	case WM_COMMAND:
		id := int(wParam & 0xFFFF)
		notif := int((wParam >> 16) & 0xFFFF)

		if notif == BN_CLICKED {
			switch id {
			case IDC_BROWSE_BTN:
				folder := browseFolder()
				if folder != "" {
					procSetWindowText.Call(hWndPathEdit, uintptr(unsafe.Pointer(utf16(folder))))
				}
			case IDC_INSTALL_BTN:
				go doInstall()
			case IDC_CANCEL_BTN:
				procDestroyWindow.Call(hwnd)
			}
		}
		return 0

	case WM_CTLCOLORSTATIC:
		return whiteBrush

	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

// ══════════════════════════════════════════════════════════════════════
// Folder browser (via PowerShell — reliable, modern dialog)
// ══════════════════════════════════════════════════════════════════════

func browseFolder() string {
	script := `Add-Type -AssemblyName System.Windows.Forms; ` +
		`$f = New-Object System.Windows.Forms.FolderBrowserDialog; ` +
		`$f.Description = 'Choose install folder for Hifeh Tasty Kitchen'; ` +
		`$f.ShowNewFolderButton = $true; ` +
		`if ($f.ShowDialog() -eq 'OK') { $f.SelectedPath } else { '' }`

	// -STA: the WinForms FolderBrowserDialog requires an STA thread;
	// without it the dialog can fail to appear or hang the call.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", script)
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ══════════════════════════════════════════════════════════════════════
// Installation logic
// ══════════════════════════════════════════════════════════════════════

func doInstall() {
	folder := getWindowText(hWndPathEdit)
	if folder == "" {
		msgBox(hWndMain, "Error", "Please choose an install folder.", 0x30)
		return
	}

	// Disable controls during install
	procEnableWindow.Call(hWndInstall, 0)
	procEnableWindow.Call(hWndCancel, 0)

	appName := "Hifeh Tasty Kitchen"

	// Kill any running instance
	setStatus("Stopping any running instance...")
	taskkill := exec.Command("taskkill", "/F", "/IM", "posd.exe")
	hideConsole(taskkill)
	taskkill.Run()

	// Create folder
	setStatus("Creating install folder...")
	if err := os.MkdirAll(folder, 0755); err != nil {
		setStatus("Error: " + err.Error())
		msgBox(hWndMain, "Error", "Could not create folder:\n\n"+err.Error(), 0x10)
		procEnableWindow.Call(hWndInstall, 1)
		procEnableWindow.Call(hWndCancel, 1)
		return
	}

	// Write posd.exe
	setStatus("Installing POS server...")
	destExe := filepath.Join(folder, "posd.exe")
	if err := os.WriteFile(destExe, embeddedPOS, 0755); err != nil {
		setStatus("Error: " + err.Error())
		msgBox(hWndMain, "Error", "Could not write posd.exe:\n\n"+err.Error(), 0x10)
		procEnableWindow.Call(hWndInstall, 1)
		procEnableWindow.Call(hWndCancel, 1)
		return
	}

	// Write uninstaller
	setStatus("Installing uninstaller...")
	destUninstall := filepath.Join(folder, "uninstall.exe")
	if err := os.WriteFile(destUninstall, embeddedUninstaller, 0755); err != nil {
		setStatus("Warning: Could not write uninstaller")
	}

	// Write App Icon
	iconPath := filepath.Join(folder, "app.ico")
	_ = os.WriteFile(iconPath, embeddedIcon, 0644)

	// Write README
	setStatus("Writing documentation...")
	readme := "# Hifeh Tasty Kitchen - POS System\r\n\r\n" +
		"Your restaurant's point of sale. Works offline, no internet needed.\r\n\r\n" +
		"## How to Start\r\n" +
		"1. Double-click the 'Hifeh Tasty Kitchen' shortcut on your Desktop\r\n" +
		"2. Open Google Chrome and go to: http://localhost:8080\r\n" +
		"3. Enter your PIN to log in\r\n\r\n" +
		"## Default PINs\r\n" +
		"- Owner:           123456\r\n" +
		"- Manager (Ada):   234567\r\n" +
		"- Cashier (Amaka): 345678\r\n" +
		"- Cashier (Tunde): 456789\r\n\r\n" +
		"## Important\r\n" +
		"- Do NOT close the black terminal window while using the system\r\n" +
		"- Do NOT delete the pos.db file - it contains all your data\r\n"
	os.WriteFile(filepath.Join(folder, "README.txt"), []byte(readme), 0644)

	// Desktop shortcut
	if isChecked(hWndChkDesktop) {
		setStatus("Creating Desktop shortcut...")
		createShortcut(
			filepath.Join(os.Getenv("USERPROFILE"), "Desktop", appName+".lnk"),
			destExe, folder, appName+" POS System", iconPath,
		)
	}

	// Start Menu shortcut
	if isChecked(hWndChkStartMenu) {
		setStatus("Creating Start Menu shortcut...")
		smDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", appName)
		os.MkdirAll(smDir, 0755)
		createShortcut(
			filepath.Join(smDir, appName+".lnk"),
			destExe, folder, appName+" POS System", iconPath,
		)
		// Also add an uninstall shortcut
		createShortcut(
			filepath.Join(smDir, "Uninstall "+appName+".lnk"),
			destUninstall, folder, "Uninstall "+appName, iconPath,
		)
	}

	// Firewall rule
	if isChecked(hWndChkFirewall) {
		setStatus("Adding firewall rule...")
		netsh := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			"name="+appName, "dir=in", "action=allow",
			"program="+destExe, "enable=yes", "profile=private",
		)
		hideConsole(netsh)
		netsh.Run()
	}

	// Register in Add/Remove Programs
	setStatus("Registering application...")
	registerUninstaller(appName, folder, destExe, destUninstall)

	// Done!
	setStatus("Installation complete!")
	msgBox(hWndMain, "Installation Complete!",
		fmt.Sprintf(
			"%s has been installed successfully!\n\n"+
				"Location: %s\n\n"+
				"Default Owner PIN: 123456\n\n"+
				"To uninstall, use 'Add or Remove Programs' in Windows Settings.",
			appName, folder,
		), 0x40)

	// Launch
	if isChecked(hWndChkLaunch) {
		cmd := exec.Command(destExe)
		cmd.Dir = folder
		hideConsole(cmd)
		cmd.Start()
		openBrowserLater()
	}

	// Close the window from the worker thread. DestroyWindow must be called
	// from the thread that created the window, so route WM_CLOSE to it.
	procSendMessage.Call(hWndMain, WM_CLOSE, 0, 0)
}

func createShortcut(lnkPath, targetExe, workDir, description, iconPath string) {
	script := fmt.Sprintf(
		`$ws = New-Object -ComObject WScript.Shell; `+
			`$s = $ws.CreateShortcut('%s'); `+
			`$s.TargetPath = '%s'; `+
			`$s.WorkingDirectory = '%s'; `+
			`$s.Description = '%s'; `+
			`$s.IconLocation = '%s,0'; `+
			`$s.Save()`,
		lnkPath, targetExe, workDir, description, iconPath,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	hideConsole(cmd)
	cmd.Run()
}

func registerUninstaller(appName, installDir, exePath, uninstallPath string) {
	keyPath := `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\HifehTastyKitchen`
	keyPtr := utf16(keyPath)

	var hKey uintptr
	var disposition uint32
	ret, _, _ := procRegCreateKeyEx.Call(
		HKEY_CURRENT_USER,
		uintptr(unsafe.Pointer(keyPtr)),
		0, 0, 0, KEY_ALL_ACCESS, 0,
		uintptr(unsafe.Pointer(&hKey)),
		uintptr(unsafe.Pointer(&disposition)),
	)
	if ret != 0 {
		return
	}
	defer procRegCloseKey.Call(hKey)

	setRegString(hKey, "DisplayName", appName)
	setRegString(hKey, "UninstallString", `"`+uninstallPath+`"`)
	setRegString(hKey, "InstallLocation", installDir)
	setRegString(hKey, "DisplayIcon", exePath+",0")
	setRegString(hKey, "Publisher", appName)
	setRegString(hKey, "DisplayVersion", "1.0.0")
	setRegDWORD(hKey, "NoModify", 1)
	setRegDWORD(hKey, "NoRepair", 1)

	// Estimated size in KB
	size := uint32(len(embeddedPOS)/1024 + len(embeddedUninstaller)/1024)
	setRegDWORD(hKey, "EstimatedSize", size)
}

func setRegString(hKey uintptr, name, value string) {
	namePtr := utf16(name)
	valUTF16, _ := syscall.UTF16FromString(value)
	procRegSetValueEx.Call(hKey,
		uintptr(unsafe.Pointer(namePtr)),
		0, REG_SZ,
		uintptr(unsafe.Pointer(&valUTF16[0])),
		uintptr(len(valUTF16)*2),
	)
}

func setRegDWORD(hKey uintptr, name string, value uint32) {
	namePtr := utf16(name)
	procRegSetValueEx.Call(hKey,
		uintptr(unsafe.Pointer(namePtr)),
		0, REG_DWORD,
		uintptr(unsafe.Pointer(&value)),
		4,
	)
}

// ══════════════════════════════════════════════════════════════════════
// Entry point
// ══════════════════════════════════════════════════════════════════════

func main() {
	// Default install path
	defaultPath = filepath.Join(os.Getenv("LOCALAPPDATA"), "HifehTastyKitchen")

	// Enable modern themed controls
	enableVisualStyles()

	hInstance, _, _ = procGetModuleHandle.Call(0)

	// Register window class
	className := utf16("HifehSetupClass")
	cursor, _, _ := procLoadCursor.Call(0, IDC_ARROW)
	icon, _, _ := procLoadIcon.Call(0, IDI_APPLICATION)

	wcx := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         0x0003, // CS_HREDRAW | CS_VREDRAW
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		HIcon:         icon,
		HCursor:       cursor,
		HbrBackground: COLOR_BTNFACE + 1,
		LpszClassName: className,
		HIconSm:       icon,
	}
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wcx)))

	// Window size
	const winW, winH = 540, 420

	// Center on screen
	screenW, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	screenH, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	x := (int(screenW) - winW) / 2
	y := (int(screenH) - winH) / 2

	// Create main window
	hwnd, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16("Hifeh Tasty Kitchen — Setup"))),
		WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX,
		uintptr(x), uintptr(y), winW, winH,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		msgBox(0, "Error", "Failed to create window.", 0x10)
		return
	}

	procShowWindow.Call(hwnd, 1) // SW_SHOWNORMAL
	procUpdateWindow.Call(hwnd)

	// Message loop
	var msg MSG
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
