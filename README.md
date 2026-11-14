# Hifeh Tasty Kitchen — POS System

<p align="center">
  <img src="web/static/images/logo.png" alt="Hifeh Tasty Kitchen" width="120">
</p>

Your restaurant's point of sale — take orders, print receipts, and track sales. Works offline, no internet needed.

---

## ⚡ Quick Setup (Recommended)

Just run the setup file — no extra software or installation tools needed:

1. **Double-click `HifehTastyKitchen_Setup.exe`**
2. Choose where to install (or use the default `%LOCALAPPDATA%\HifehTastyKitchen`)
3. Choose your options:
   - ☑ **Desktop Shortcut** (adds a branded icon to your desktop)
   - ☑ **Start Menu Shortcut**
   - ☑ **Windows Firewall Rule** (enables customer display and other cashier terminals over local Wi-Fi)
   - ☑ **Launch after install**
4. Click **Install** — Chrome will automatically open to **`http://localhost:8080`**!

> **Portable Option:** If you prefer running without an installer, you can still just double-click `posd.exe` directly in any folder.

---

## 🔑 Default PINs

| Who | PIN | Role & Access |
|---|---|---|
| **Owner** | `123456` | Full system control, financial audit trail, staff management |
| **Manager Ada** | `234567` | Kitchen restock, shift sales, order edits/voids |
| **Cashier Amaka** | `345678` | Clean till interface (take orders & accept cash) |
| **Cashier Tunde** | `456789` | Clean till interface (take orders & accept cash) |

⚠️ **Change these PINs** after your first login in Admin Console → Staff & Workers.

---

## 🖥️ Setting Up the Customer Screen

You can show customers what they're ordering on a second screen (TV or tablet).

1. When `posd.exe` starts, check the address shown (or use your PC's local IP address):
   ```
   http://192.168.1.50:8080/display
   ```
2. Open that link on the TV or tablet browser
3. It will mirror orders in real time via live Server-Sent Events (SSE)

> The TV/tablet must be on the **same Wi-Fi or router network** as the POS computer.

---

## 🍽️ Taking Orders & In-Place Editing

1. **New Order**:
   - Log in with your Cashier PIN
   - Pick **Dine-in** or **Takeaway**
   - Tap dishes to add them, adjust quantities using **+** / **−**
   - Tap **"Show on display"** to mirror to customer
   - Click **"Pay (cash)"** and choose whether to print receipt

2. **In-Place Order Editing**:
   - If a manager reopens a paid order from Recent Orders, an **"EDITING"** badge appears on the till
   - Cashiers or managers can add/remove items in place
   - The button transforms into **"Save Changes 💾"**
   - Receipts reprint with an explicit `*** UPDATED RECEIPT (EDITED) ***` header, and an `edited` log entry is recorded in the immutable audit trail

---

## 🍔 Meal Combos

Bundles such as "Rice + Chicken + Coke" can be created once and sold with one tap:

1. **Creating a combo** (Owner Console → Menu):
   - Tick **"This is a combo"** in the dish form
   - Pick the included dishes and their quantities, then **Add component**
   - The combo has its own price — stock tracking lives on the components, not the combo

2. **Selling a combo**:
   - Tapping a combo adds the combo at its price plus the bundled dishes listed as *free* items (`included in combo` on the receipt)
   - Raising/lowering the combo quantity moves the bundled items along with it; removing the combo removes them too
   - If any bundled dish is sold out, the combo is blocked until restocked, and paying a sale deducts each component's stock


---

## 📦 Global Real-Time Inventory & Stock

Stock is synchronized live across all cashiers and kitchen managers:

- **Automatic depletion**: Completed sales immediately deduct quantities from global inventory.
- **Out-of-stock lock**: When an item reaches 0, all cashier screens display **"Sold Out"** and block additions.
- **Decrement & Increment adjustments**: Kitchen managers have quick stock controls (`-10`, `-5`, `+10`, `+25`, `+50`, and `Set Stock`).
- **Low stock indicators**: Dishes show live quantity badges (e.g. `12 left`, `Sold Out`) directly on the till cards.

---

## 👥 Roles & Capabilities

| What They Can Do | Cashier (Amaka / Tunde) | Manager (Ada) | Owner |
|---|:---:|:---:|:---:|
| **Take Orders & Accept Cash** | ✅ | ✅ | ❌ |
| **Clean Till Interface (No button clutter)** | ✅ | — | — |
| **View Recent Orders & Reprint Receipts** | ❌ (hidden) | ✅ (direct) | ✅ |
| **Edit or Void Existing Orders** | ❌ | ✅ (in-place) | ✅ |
| **Restock Kitchen Inventory (+ / − / Set)** | ❌ (hidden) | ✅ (direct) | ✅ |
| **Toggle Dish On/Off (Available/Unavailable)** | ❌ | ✅ | ✅ |
| **View Shift Sales & Cashier Breakdown** | ❌ | ✅ (Manager Hub) | ✅ |
| **Manage Staff (Add/Remove Workers, Reset PINs)** | ❌ | ❌ | ✅ (Owner Console) |
| **Change Store Settings, VAT, Logo** | ❌ | ❌ | ✅ (Owner Console) |
| **View Permanent Financial Audit Trail** | ❌ | ❌ | ✅ (Owner Console) |

---

## 🖨️ Setting Up Your Thermal Printer

Compatible with standard **80mm thermal receipt printers**:

1. Install your printer drivers on Windows normally
2. In Google Chrome's print dialog:
   - Select your thermal printer
   - Set Margins to **None**
   - Untick **Headers and footers**
3. Print test receipt directly from the till

---

## 🗑️ Uninstalling

If installed using `HifehTastyKitchen_Setup.exe`:

1. Open Windows **Settings → Apps → Installed apps** (or "Add or Remove Programs")
2. Find **Hifeh Tasty Kitchen** and click **Uninstall**
3. The uninstaller will ask:
   - **Keep sales data?** You can safely preserve `pos.db` in case you reinstall later, or choose to delete everything.
   - All shortcuts, firewall rules, and app executables are automatically removed.

---

## 💾 Backing Up Your Data

All orders, inventory, dishes, and staff data reside in **`pos.db`** (stored in your installation folder).

- To back up, simply copy **`pos.db`** to a USB flash drive or backup drive.
- To restore, place `pos.db` back into the app folder before launching `posd.exe`.

---

## 🛠️ Building From Source

Prerequisites: [Go 1.22+](https://golang.org)

```bash
# Build standalone Linux binary
go build -o posd ./cmd/posd

# Build standalone Windows binary
GOOS=windows GOARCH=amd64 go build -o posd.exe ./cmd/posd

# Build the complete Windows GUI installer with embedded icons
./build_installer.sh
```

---

© Hifeh Tasty Kitchen
