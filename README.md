# Local Archive 📁

A minimalist, offline-first local web application for archiving incoming and outgoing administrative documents, letters, and official records (PDFs and images). Built with pure Go and SQLite, featuring instant multi-field search, full archive backups (database + attachments), one-click restoration, PWA desktop support, and a standalone Windows installer.

---

## ✨ Features

- **Document Management:** Dedicated workflows for both **Incoming Documents** and **Outgoing Documents**.
- **Multi-Field Instant Search:** Fast full-text search across reference numbers, issuing entities, subjects, and dates.
- **Embedded & Pure Go:** Zero CGO dependencies. HTML templates, CSS, JS, and icons are embedded directly into a single binary.
- **Automated Backup & Restore:** Generate complete ZIP backups containing the database and uploaded attachments; restore with integrity checks.
- **Progressive Web App (PWA):** Install as a standalone desktop application directly from modern Chromium browsers.
- **Native Windows Installer:** Self-contained executable installer (`LocalArchive-Setup.exe`) that configures shortcuts, autostart, and clean uninstallation.

---

## 🚀 Setup & Installation Steps

### Option A: Windows Installer (Recommended for End Users)

No development tools, Go runtime, or command-line steps are needed.

1. **Download the Installer:**
   - Go to the **[Releases](https://github.com/sadiqAlAboudi/local-archive/releases)** page.
   - Download the latest **`LocalArchive-Setup.exe`**.

2. **Run the Installer:**
   - Double-click **`LocalArchive-Setup.exe`**.
   - The installer automatically:
     - Installs the application to `%LOCALAPPDATA%\LocalArchive`.
     - Creates Desktop and Start Menu shortcuts with the application icon.
     - Registers the application in the Windows Registry to start on boot in the background.
     - Registers an entry in Windows **Installed Apps / Programs & Features** for clean uninstallation.
     - Launches the application and opens your default browser to **http://localhost:8080**.

3. **Initial Sign-In:**
   - **Default Username:** `admin`
   - **Default Password:** `admin`
   - *On first login, the application will prompt you to set a secure custom username and password.*

4. **Uninstallation:**
   - Go to Windows **Settings > Apps > Installed Apps**, locate **Local Archive**, and click **Uninstall** (or run `uninstall.exe` in `%LOCALAPPDATA%\LocalArchive`).
   - Your archived documents and database are preserved safely to avoid accidental data loss.

---

### Option B: Running from Source (Development / Linux / macOS)

#### Prerequisites
- **Go 1.22+** installed on your system.

#### Steps

1. **Clone the repository:**
   ```bash
   git clone git@github.com:sadiqAlAboudi/local-archive.git
   cd local-archive
   ```

2. **Download dependencies:**
   ```bash
   go mod download
   ```

3. **Start the application:**
   ```bash
   go run main.go
   ```

4. **Access the Web Interface:**
   - Open **[http://localhost:8080](http://localhost:8080)** in your browser.
   - Log in using `admin` / `admin` and configure your credentials.

#### Available Command-Line Flags
```bash
go run main.go -port=9090          # Run server on a custom port (default: 8080)
go run main.go -no-browser         # Start without automatically opening a browser window
go run main.go -install            # Register in Windows Startup folder for automatic boot launch
go run main.go -uninstall          # Remove from Windows Startup folder
```

---

## 🔨 Building the Windows Installer

You can compile the Windows installer directly from Linux, macOS, or Windows:

```bash
chmod +x build-windows.sh
./build-windows.sh
```

This script:
1. Compiles `archive.exe` with `CGO_ENABLED=0 GOOS=windows GOARCH=amd64`.
2. Packages `archive.exe` and the application icon into the native installer payload (`cmd/installer/payload`).
3. Compiles `cmd/installer` into **`LocalArchive-Setup.exe`** with a hidden GUI window flag (`-H windowsgui`).
4. Cleans up intermediate payload binaries, leaving only `LocalArchive-Setup.exe`.

---

## 🤖 Automated CI/CD & Releases

This repository includes a GitHub Actions workflow ([`.github/workflows/release.yml`](.github/workflows/release.yml)) that automates building and publishing releases.

### How to Publish a New Release:
1. Commit your changes and push to `main`:
   ```bash
   git push origin main
   ```

2. Create a version tag and push it:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. GitHub Actions will automatically:
   - Check out the repository.
   - Set up the Go environment.
   - Run `build-windows.sh` to compile `LocalArchive-Setup.exe`.
   - Publish a new GitHub Release with `LocalArchive-Setup.exe` attached and auto-generated release notes.

You can also trigger builds manually from the **Actions** tab on GitHub using the **Run workflow** button.

---

## 📋 Document Types & Fields

### 1. Incoming Documents (الكتب الواردة)
- **Document Type:** Incoming Document
- **Serial Number:** Unique registry/archive sequence number
- **Registration Date:** Date document was received and archived
- **Issuing Entity:** Government body, company, or department that issued the document
- **Letter Number:** Official number printed on the original letter
- **Letter Date:** Date printed on the original letter
- **Subject:** Brief title or summary
- **Attachment:** Attached PDF file or scanned image
- **Automatic Download Naming:** `[Serial]-[Subject].[ext]`

### 2. Outgoing Documents (الكتب الصادرة)
- **Document Type:** Outgoing Document
- **Issue Number:** Organization issue/letter sequence number
- **Issue Date:** Date of issuance
- **Destination Entity:** Receiving department or organization
- **Subject:** Brief title or summary
- **Attachment:** Attached PDF file or scanned image
- **Automatic Download Naming:** `[IssueNumber]-[Subject].[ext]`

---

## 💾 Storage, Backups & Data Protection

- **Local Storage Path:** Data is stored under `data/` (or platform user data directory):
  - `data/archive.db`: SQLite database in WAL (Write-Ahead Logging) mode.
  - `data/uploads/`: Attached PDF and image files.
- **Full Backup:**
  - Click **"Full Backup"** in the top navigation bar.
  - Generates a timestamped `.zip` containing the SQLite database and all attachment files.
- **Restoration:**
  - Click **"Restore Backup"** in the top navigation bar and select a valid backup `.zip` or `.db` file.
  - The application validates database integrity before replacing existing data and automatically reloads.

---

## 💻 Progressive Web App (PWA) Support

Local Archive includes a Web App Manifest and Service Worker:
1. Open **http://localhost:8080** in Chrome, Edge, or Brave.
2. Click the **Install** icon in the address bar.
3. The application runs in a dedicated desktop window without browser bars, complete with application icons.

---

## 🏗️ Project Architecture

```
local-archive/
├── .github/
│   └── workflows/
│       └── release.yml           # Automated release workflow (GitHub Actions)
├── cmd/
│   └── installer/                # Standalone Windows native installer source
│       ├── installer_windows.go  # Windows installation logic, registry, shortcuts
│       ├── installer_other.go    # Stub for non-Windows targets
│       └── main.go               # Installer entry point & CLI flags
├── internal/
│   ├── backup/                   # ZIP backup generation and archive restoration
│   ├── database/                 # SQLite connection, schema migrations, and queries
│   ├── handlers/                 # HTTP controllers, routing, and session auth
│   ├── models/                   # Document, User, Session, and View data models
│   └── sysutil/                  # Paths, platform helpers, and autostart utilities
├── static/                       # Static assets: CSS, JS, PWA icons, manifest
├── templates/                    # Semantic HTML templates
├── build-windows.sh              # Windows installer build script
├── go.mod                        # Go module definition
├── go.sum                        # Go module checksums
├── main.go                       # Main web server entry point
└── README.md                     # Documentation
```

---

## 🎨 Design Principles

- **Minimalist Aesthetic:** Clean, distraction-free interface built on `#000000` with high-contrast neutral surfaces.
- **Semantic HTML5:** Built using standard HTML tags (`<header>`, `<nav>`, `<main>`, `<table>`, `<dialog>`, `<form>`, `<dl>`, `<figure>`).
- **Modern Pure CSS:** Nested CSS without third-party frameworks or utility classes.
- **Responsive & RTL Compatible:** Fluid layout that adapts across desktop and tablet screen sizes.
