# Tatsy — Hifeh Tasty Kitchen POS

<p align="center">
  <img src="web/static/images/logo.png" alt="Hifeh Tasty Kitchen" width="120">
</p>

A fast, offline-first Point of Sale (POS) system built for Nigerian restaurants and food businesses. Single binary, no internet required — runs on any laptop or mini PC behind the counter.

---

## ✨ Features

| Feature | Description |
|---|---|
| **Till (Cashier Screen)** | Tap-to-add menu grid, quantity controls, dine-in / takeaway toggle, table numbers |
| **Customer Display** | Real-time order mirror on a second screen (TV/tablet), shows items as they're added, full-screen payment confirmation overlay |
| **Thermal Receipt Printing** | 80mm receipt format via browser `@media print`, includes order number, items, totals, business details |
| **Reprint Receipts** | Reprint any recent receipt directly from the Recent Orders list |
| **Admin Console** | Manage menu items, categories, staff, settings, upload images, view sales reports |
| **Manager PIN Auth** | Edit or void recent paid orders with manager/owner PIN authorization — logged to audit trail |
| **Image Management** | Upload dish photos and logo via admin, auto-compressed client-side before upload, SVG fallbacks |
| **Live Updates (SSE)** | Changes propagate instantly to customer display via Server-Sent Events |
| **Roles & Permissions** | Owner, Manager, and Cashier roles with granular capability controls |
| **VAT / Tax** | Configurable VAT (inclusive or exclusive), packaging fee for takeaway orders |
| **Audit Trail** | Immutable log of all corrections, voids, reprints — visible to Owner only |

---

## 🚀 Quick Start

### Prerequisites

- A modern web browser (Chrome, Firefox, Edge)
- Optional: thermal receipt printer (80mm)

### Getting Started (Windows)

1. **Double-click `posd.exe`** — the server starts immediately
2. Open your browser and go to **`http://localhost:8080`**
3. Log in with a PIN (see default PINs below)

> **💡 Windows Tips:**
> - If you get a **Windows Firewall** prompt, click **"Allow access"** so other devices on your network (customer display, etc.) can connect.
> - To keep the server running, don't close the terminal window that appears.
> - You can create a shortcut to `posd.exe` on the Desktop for quick launch.
> - To run on startup: press `Win + R`, type `shell:startup`, and place a shortcut to `posd.exe` there.
> - To stop the server, press `Ctrl + C` in the terminal or close the window.

### Getting Started (Linux)

```bash
# Make it executable (first time only)
chmod +x posd

# Run the POS server
./posd
```

Then open **`http://localhost:8080`** in your browser.

---

The server starts on **port 8080** by default. You'll see:

```
tatsy POS server running  addr=:8080  db=pos.db
open the till on this PC  url=http://localhost:8080
open the buyer display on Terminal B  url=http://<your-ip>:8080/display
```

The database file (`pos.db`) is created automatically on first run.

### Configuration (Optional)

You can customize the port and database path:

```bash
# Linux / macOS
./posd -addr :9090 -db mystore.db

# Windows
.\posd.exe -addr :9090 -db mystore.db
```

---

## 🔐 Default Login PINs

| Role | Name | PIN |
|---|---|---|
| **Owner** | Owner | `123456` |
| **Manager** | Manager Ada | `234567` |
| **Cashier** | Amaka | `345678` |
| **Cashier** | Tunde | `456789` |

> **⚠️ Change these PINs immediately** in the Admin Console → Staff & Workers after first login.

---

## 📱 Pages & URLs

| Page | URL | Who Uses It |
|---|---|---|
| **Login** | `/login` | Everyone |
| **Till** | `/` | Cashiers, Managers |
| **Customer Display** | `/display` | Customer-facing screen (TV/tablet) |
| **Admin Console** | `/admin` | Owner (Manager sees limited version) |

### Typical Setup

1. **Counter PC** — Open the Till at `http://localhost:8080` (cashier logs in)
2. **Customer Screen** — Open `http://<server-ip>:8080/display` on a second monitor, TV, or tablet facing the customer
3. **Back Office** — Owner logs in and opens `/admin` for management

---

## 🍽️ How to Use the Till

### Taking an Order

1. **Log in** with your PIN
2. **Select order type**: Dine-in or Takeaway (optionally set table number)
3. **Tap menu items** to add them to the order — quantities update automatically
4. Use **+** / **−** buttons to adjust quantities, or remove items
5. Click **"Show on display"** to push the order to the customer screen
6. Click **"Pay (cash)"** to complete the payment

### After Payment

- A **confirmation modal** appears showing the order summary
- Choose **"Yes, Print Receipt"** to print via thermal printer, or **"No Thanks"** to start a new order
- The customer display automatically shows a payment success overlay

### Reprinting a Receipt

1. Click the **"Recent"** button (clock icon) in the Till header
2. Find the paid order and click **🖨️ Reprint**
3. The browser print dialog opens with the receipt

### Editing / Voiding a Paid Order

1. Click **"Recent"** → find the order
2. Click **"Edit Order"** or **"Void"**
3. Enter a **Manager or Owner PIN** and a reason
4. The action is logged to the audit trail

---

## ⚙️ Admin Console

Access at `/admin` (Owner login required).

### Tabs

| Tab | What You Can Do |
|---|---|
| **Overview & Sales** | Today's revenue, order count, top items, per-cashier breakdown |
| **Menu Items** | Add, edit, toggle active, delete dishes/drinks; upload photos |
| **Categories** | Create, rename, delete menu categories |
| **Staff & Workers** | Add users, assign roles, reset PINs, activate/deactivate |
| **Settings** | Business name, receipt footer, VAT config, packaging fee, logo upload |
| **Audit Trail** *(Owner only)* | View all corrections, voids, reprints with timestamps and reasons |

### Uploading Images

- **Dish photos**: Edit a menu item → click the image area → select a photo
- **Logo**: Settings tab → Upload Logo section
- Images are **automatically compressed** client-side (resized to max 400×400 for dishes, 200×200 for logo) before upload
- If no image is uploaded, a **default SVG placeholder** is shown

---

## 🏗️ Architecture

```
tatsy/
├── cmd/posd/           # Entry point — starts HTTP server
├── internal/
│   ├── api/            # HTTP handlers, routing, SSE hub, sessions
│   ├── core/           # Roles & capabilities (Owner, Manager, Cashier)
│   └── store/          # SQLite data layer — orders, menu, users, audit, settings
├── web/
│   ├── static/         # CSS, JS, images (embedded + disk overlay for uploads)
│   └── templates/      # Go HTML templates (seller, display, admin, login)
├── go.mod
└── pos.db              # SQLite database (created on first run)
```

### Tech Stack

- **Backend**: Go standard library (`net/http`, `html/template`, `database/sql`)
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Frontend**: Vanilla HTML/CSS/JS — no frameworks, no build step
- **Real-time**: Server-Sent Events (SSE)
- **Auth**: Argon2id PIN hashing (`golang.org/x/crypto/argon2`)
- **Static files**: Embedded via `embed.FS`, disk overlay for uploaded images

### Data Flow

```
Cashier (Till)  →  POST /api/order/*  →  Go Server  →  SQLite
                                           ↓ SSE
                                     Customer Display
```

---

## 🖨️ Receipt Printing

Receipts are formatted for **80mm thermal printers** using CSS `@media print`. To print:

1. When the browser print dialog appears, select your thermal printer
2. Set margins to **None** and page size to **80mm** (or the closest option)
3. Disable headers/footers in the print settings

### Chrome Tips

- Go to `chrome://settings/printing` to set your default printer
- For a kiosk setup, you can use Chrome's `--kiosk-printing` flag to auto-print without the dialog

---

## 🔒 Roles & Permissions

| Capability | Cashier | Manager | Owner |
|---|:---:|:---:|:---:|
| Take orders & accept payments | ✅ | ✅ | ❌ |
| Edit/void orders (with manager PIN) | — | ✅ (authorizer) | ✅ (authorizer) |
| Access Admin Console | ❌ | ❌ | ✅ |
| Manage menu, staff, settings | ❌ | ❌ | ✅ |
| View audit trail | ❌ | ❌ | ✅ |

> **Note**: The Owner role is for administration only and cannot operate the till. Managers can sell and authorize corrections but cannot access the full admin console.

---

## 📦 Deployment Tips

### Run as a System Service (Linux)

Create `/etc/systemd/system/tatsy.service`:

```ini
[Unit]
Description=Tatsy POS Server
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/tatsy
ExecStart=/opt/tatsy/posd
Restart=always
User=pos

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable tatsy
sudo systemctl start tatsy
```

### Run on Windows Startup

**Option A — Startup Folder (simplest):**
1. Press `Win + R`, type `shell:startup`, press Enter
2. Create a shortcut to `posd.exe` and place it in the folder that opens
3. The POS server will start automatically every time Windows boots

**Option B — Task Scheduler (runs without a visible window):**
1. Open **Task Scheduler** → Create Basic Task
2. Name it "Tatsy POS", trigger: "When the computer starts"
3. Action: Start a Program → browse to `posd.exe`
4. Set "Start in" to the folder containing `posd.exe`

### Backup

The entire state is in **one file**: `pos.db`. Back it up regularly:

```bash
# Linux / macOS
cp pos.db pos-backup-$(date +%Y%m%d).db
```

```powershell
# Windows (PowerShell)
Copy-Item pos.db "pos-backup-$(Get-Date -Format yyyyMMdd).db"
```

### Network Access

To access the till and customer display from other devices on the same Wi-Fi / LAN:

**Linux:**
```bash
sudo firewall-cmd --add-port=8080/tcp --permanent
sudo firewall-cmd --reload
```

**Windows:**
- When you first run `posd.exe`, Windows Firewall will prompt you — click **"Allow access"**
- If you missed the prompt: Control Panel → Windows Defender Firewall → Allow an app → Add `posd.exe`

---

## 📄 License

Private / Proprietary — Hifeh Tasty Kitchen.
