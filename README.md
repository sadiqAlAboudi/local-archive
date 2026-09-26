# Local Archive 📁

A clean, minimalist, offline local web application for archiving official documents, letters, and records (PDFs and images) with SQLite storage, quick search, retrieval, printing, and automated database backups.

---

## 🚀 Quick Start (Development / Linux)

Run the server with:

```bash
go run main.go
```
*(or `go run main.go serve`)*

Then open your browser to **[http://localhost:8080](http://localhost:8080)**.

### Initial Credentials
- **Username:** `admin`
- **Password:** `admin`

> **First Login Notice:** For security, the system will prompt you immediately after your first login to set your own custom username and password before accessing the archive.

---

## 📋 Document Metadata Fields

Each archived document saves the following fields:
1. **رقم التسلسل** (Serial Number)
2. **التاريخ** (Date)
3. **اسم الدائرة** (Department Name)
4. **رقم الكتاب** (Letter / Document Reference Number)
5. **تاريخ الكتاب** (Letter Date)
6. **الموضوع** (Subject)
7. **Attached File** (PDF document or Image: JPG, PNG, WEBP)

- **Custom Download Naming:** When downloading any document, it is automatically named `[رقم التسلسل]-[الموضوع].[extension]`.
- **Multi-Field Search:**
  - **Quick Multi-Word Search:** Type multiple words in the search box (e.g. `التربية 2026` or `101 نقل`) and the system automatically matches records across multiple fields simultaneously.
  - **Custom Field Filtering:** Click **"تصفية مخصصة حسب عدة حقول"** to filter specifically by Serial Number, Department, Letter Number, Date, or Subject.

---

## 💻 PWA Desktop App Installation

Local Archive is fully configured as a Progressive Web App (PWA):
- Open **http://localhost:8080** in Google Chrome, Microsoft Edge, or Brave.
- Click the **Install** button (icon in the browser URL/address bar) or choose **"Install Local Archive"** / **"تثبيت التطبيق"** from the browser menu.
- The app will install directly to your desktop and run in its own clean, dedicated desktop window with its custom archive icon.

## 🪟 Windows Deployment & Boot Startup (For Non-Technical Users)

A pre-compiled, self-contained Windows 64-bit executable `archive.exe` is included in this repository. No installation of Go, databases, or third-party tools is required.

### 1. Manual Launch
- Double-click **`start.bat`** (or `archive.exe`).
- The application will start and automatically open your default browser to the archive.

### 2. Run Automatically on Windows Boot (Autostart)
- Double-click **`install-startup.bat`**.
- It creates a silent launcher in your Windows Startup directory (`%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\LocalArchive.vbs`).
- **Result:** Whenever Windows starts up, Local Archive runs quietly in the background without showing any command prompt window. You can access it anytime at `http://localhost:8080`.

### 3. Remove Automatic Startup
- Double-click **`uninstall-startup.bat`** to remove Local Archive from Windows startup.

---

## 💾 Data Folder & Backup

- **Automatic Folder Generation:** If the `data` folder is empty or does not exist, running the application automatically creates `data/`, `data/uploads/`, and initializes the SQLite database `archive.db`.
- **Database Backup:** Click the **"نسخ احتياطي للقاعدة"** (Backup Database) button in the top navigation bar to download a snapshot of the current SQLite database (`archive_backup_*.db`).

---

## 🎨 Design & Rules Compliance

- **Arabic Interface (RTL):** Fully designed in Arabic with right-to-left layout.
- **Minimalist Black Theme:** Clean `#000000` background with neutral dark surfaces and high-contrast typography.
- **Semantic HTML Only:** Built using native tags (`<header>`, `<nav>`, `<main>`, `<table>`, `<dialog>`, `<form>`, `<dl>`, `<figure>`).
- **Nested CSS & Zero Custom Classes:** Pure CSS nesting without a single `class="..."` anywhere in the codebase.
