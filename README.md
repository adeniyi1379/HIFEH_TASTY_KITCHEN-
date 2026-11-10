# Hifeh Tasty Kitchen — POS System

<p align="center">
  <img src="web/static/images/logo.png" alt="Hifeh Tasty Kitchen" width="120">
</p>

Your restaurant's point of sale — take orders, print receipts, and track sales. Works offline, no internet needed.

---

## 📦 What's Inside

You only need **one file**: `posd.exe`. Everything is packed inside it.

When you run it for the first time, it creates a `pos.db` file — this stores all your orders, menu, and staff data. **Do not delete this file.**

---

## 🚀 How to Start

1. **Double-click `posd.exe`**
2. A black window opens — this means it's running. **Don't close it.**
3. Open **Google Chrome** and go to: **`http://localhost:8080`**
4. Enter your PIN to log in

That's it — you're ready to sell!

> If Windows asks "Do you want to allow this app?" — click **Yes / Allow access**.

---

## 🔑 Default PINs

| Who | PIN |
|---|---|
| **Owner** | `123456` |
| **Manager Ada** | `234567` |
| **Cashier Amaka** | `345678` |
| **Cashier Tunde** | `456789` |

⚠️ **Change these PINs** after your first login. Go to Admin Console → Staff & Workers.

---

## 🖥️ Setting Up the Customer Screen

You can show customers what they're ordering on a second screen (TV or tablet).

1. When you start `posd.exe`, look at the black window — it shows a link like:
   ```
   http://192.168.1.50:8080/display
   ```
2. Open that link on the TV or tablet browser
3. It will mirror the order in real time

> The TV/tablet must be on the **same Wi-Fi** as the POS computer.

---

## 🍽️ Taking Orders

1. Log in with your PIN
2. Pick **Dine-in** or **Takeaway**
3. Tap on dishes to add them
4. Use **+** / **−** to change quantities
5. Press **"Show on display"** to show the customer
6. Press **"Pay (cash)"** when the customer pays
7. Choose to **print a receipt** or skip

---

## 🖨️ Reprinting a Receipt

1. Click the **clock icon** (Recent) at the top
2. Find the order
3. Click **🖨️ Reprint**

---

## ✏️ Editing or Cancelling a Paid Order

1. Click the **clock icon** (Recent)
2. Click **"Edit Order"** or **"Void"**
3. A manager or owner must enter their PIN and give a reason
4. The change is recorded and cannot be erased

---

## ⚙️ Admin Console (Owner Only)

1. Log in as **Owner**
2. Click **"Console"** at the top
3. From here you can:
   - View **today's sales** and reports
   - **Add, edit, or remove dishes** from the menu
   - **Upload photos** for dishes and your logo
   - **Add or remove staff** and reset PINs
   - Change **business name**, receipt text, and tax settings
   - View the **audit trail** (who changed or voided what)

---

## 🖨️ Setting Up Your Printer

This works with **80mm thermal receipt printers** (the small ones restaurants use).

1. Install your printer on Windows normally
2. When the print window appears in Chrome, select your thermal printer
3. Set margins to **None**
4. Untick "Headers and footers"

**Tip:** In Chrome, you can set your thermal printer as the default so it prints faster.

---

## 💾 Backing Up Your Data

All your data (orders, menu, staff) is in one file: **`pos.db`**

To back up, just **copy `pos.db`** to a USB drive or another folder. Do this regularly!

---

## 🔄 Auto-Start When Computer Turns On

1. Right-click `posd.exe` → **Create shortcut**
2. Press `Win + R` on your keyboard
3. Type `shell:startup` and press Enter
4. Move the shortcut into the folder that opens
5. Done — POS will start automatically every time you turn on the computer

---

## ❓ Troubleshooting

| Problem | Solution |
|---|---|
| Browser says "can't connect" | Make sure the black window (`posd.exe`) is still open |
| Customer screen doesn't load | Make sure both devices are on the same Wi-Fi |
| Forgot a PIN | Owner can reset any PIN in Admin Console → Staff & Workers |
| Printer not working | Check it's installed in Windows Settings → Printers, then select it in Chrome's print dialog |
| "Address already in use" error | Close any other `posd.exe` that might be running (check your taskbar) |

---

© Hifeh Tasty Kitchen
