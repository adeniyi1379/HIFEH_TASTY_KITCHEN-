# Hifeh Tasty Kitchen — Windows Installer Build Guide

## Prerequisites

1. **A Windows PC** (or VM)
2. **[Inno Setup 6](https://jrsoftware.org/isdl.php)** — free download, ~5 MB install

## Folder Structure

Before compiling, your `installer/` folder should look like this:

```
tatsy/
├── posd.exe              ← the Windows binary
├── README.md
├── web/static/images/logo.png
└── installer/
    ├── setup.iss          ← the installer script
    ├── app.ico            ← app icon (see below)
    └── Output/            ← created automatically
        └── HifehTastyKitchen_Setup_v1.0.0.exe  ← the final installer
```

## Step 1: Create the App Icon (app.ico)

You need an `.ico` file for the installer and Windows shortcuts. Options:

### Option A: Online converter (easiest)
1. Go to [icoconvert.com](https://icoconvert.com/) or [convertio.co](https://convertio.co/png-ico/)
2. Upload `web/static/images/logo.png`
3. Select sizes: 16x16, 32x32, 48x48, 64x64, 128x128, 256x256
4. Download and save as `installer/app.ico`

### Option B: ImageMagick (if installed)
```bash
magick web/static/images/logo.png -define icon:auto-resize=256,128,64,48,32,16 installer/app.ico
```

### Option C: Skip it
Comment out the `SetupIconFile=app.ico` line in `setup.iss` — the installer will use the default Inno Setup icon.

## Step 2: Compile the Installer

1. Copy the entire `tatsy/` folder to your Windows PC
2. Right-click `installer/setup.iss` → **Open with Inno Setup**
3. Press **Ctrl+F9** (or Build → Compile)
4. Wait ~10 seconds
5. The installer appears at: `installer/Output/HifehTastyKitchen_Setup_v1.0.0.exe`

## What the Installer Does

| Step | Action |
|------|--------|
| Welcome screen | Shows app name + "works offline" message |
| Info page | Displays README.md content |
| Install directory | User picks folder (default: `C:\Program Files\Hifeh Tasty Kitchen`) |
| Start Menu group | User picks group name |
| Options | ☑ Desktop shortcut, ☑ Launch after install, ☑ Firewall rule |
| Install | Copies `posd.exe`, `README.md`, `logo.png` |
| Finish | Launches the server + opens browser to `http://localhost:8080` |

## What Gets Installed

- `C:\Program Files\Hifeh Tasty Kitchen\posd.exe` — the POS server
- `C:\Program Files\Hifeh Tasty Kitchen\README.md` — help file
- `C:\Program Files\Hifeh Tasty Kitchen\logo.png` — branding
- Start Menu shortcuts (launch server + open browser)
- Desktop shortcut (optional)
- Windows Firewall inbound rule for LAN access (optional)
- Uninstaller in Add/Remove Programs

## Uninstalling

- Go to **Settings → Apps → Hifeh Tasty Kitchen → Uninstall**
- Or run the uninstaller from the Start Menu
- The database (`pos.db`) is NOT deleted by default (data preservation)

## Updating the Version

Edit line 14 in `setup.iss`:
```
#define MyAppVersion    "1.1.0"
```

The installer filename updates automatically to `HifehTastyKitchen_Setup_v1.1.0.exe`.

## Tips

- **Same AppId** = upgrade install (replaces old version, keeps database)
- **Database is safe** — `pos.db` is created at runtime in the app directory, not bundled
- **Silent install** for IT: `HifehTastyKitchen_Setup_v1.0.0.exe /SILENT /DIR="C:\POS"`
