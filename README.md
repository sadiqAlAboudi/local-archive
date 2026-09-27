# Local Archive 📁

A minimalist, offline-first native cross-platform desktop application for archiving incoming and outgoing administrative documents, letters, and official records (PDFs and images). Built with **Wails v2**, pure Go, SQLite, and Vanilla HTML/CSS/JS, featuring instant multi-field search, full archive backups (database + attachments), one-click restoration, native file dialogs, and standalone desktop executables.

---

## ✨ Features

- **Native Desktop Application:** Powered by [Wails v2](https://wails.io) with direct Go runtime bindings (`window.go.main.App.*`) and embedded Webview.
- **Document Management:** Dedicated workflows for both **Incoming Documents** (الكتب الواردة) and **Outgoing Documents** (الكتب الصادرة).
- **Native OS File Dialogs:** Direct local file selection using native operating system dialogs without HTTP multipart overhead.
- **Multi-Field Instant Search:** Fast live search across reference numbers, issuing entities, subjects, and dates.
- **Zero CGO SQLite:** Embedded `modernc.org/sqlite` database stored in `%LOCALAPPDATA%\LocalArchive` on Windows (or `./data` in development mode).
- **Automated Backup & Restore:** Save full ZIP archives (database + attachments) directly to any disk location via native save dialogs, and restore with automatic database re-initialization.
- **Embedded Document Viewer:** Stream and preview attached PDFs and images securely inside the webview without exposing external HTTP ports.

---

## 🚀 Development & Build

### Prerequisites
- **Go 1.22+** installed on your system.
- **Wails v2 CLI:**
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

### Development Mode
During development, Wails hot-reloads frontend changes and recompiles Go code automatically:
```bash
wails dev
```

### Building the Desktop Executable

#### 1. Standalone Windows Executable
```bash
wails build -platform windows/amd64
```
The compiled `.exe` will be available in `build/bin/LocalArchive.exe`.

#### 2. Standalone Windows Installer (NSIS)
```bash
wails build -platform windows/amd64 -nsis
```

---

## 📂 Project Structure

```text
local-archive/
├── build/                        # Application icon and platform assets
│   ├── appicon.png               # Master icon
│   └── windows/
│       └── icon.ico              # Windows binary embedded icon
├── frontend/                     # Unified client frontend (Vanilla HTML/CSS/JS)
│   ├── app.css                   # Modern CSS styling (RTL / Arabic)
│   ├── app.js                    # Client logic and Wails bindings
│   ├── favicon.svg               # SVG application icon
│   └── index.html                # Single-page desktop interface
├── internal/
│   ├── backup/                   # ZIP backup generation and archive restoration
│   ├── database/                 # SQLite connection, schema migrations, and queries
│   ├── models/                   # Document, User, Session, and View data models
│   └── sysutil/                  # Persistent data paths (%LOCALAPPDATA%) and utilities
├── app.go                        # Wails App struct and exposed backend methods
├── app_test.go                   # Unit tests for Wails App bindings
├── go.mod                        # Go module definition
├── go.sum                        # Go module checksums
├── main.go                       # Wails entry point, window properties, and file server
├── wails.json                    # Wails v2 project configuration
└── README.md                     # Documentation
```

---

## 🎨 Design Principles

- **Minimalist Aesthetic:** Clean, distraction-free interface built on `#000000` and high-contrast surfaces.
- **Semantic HTML5:** Built using standard HTML tags (`<header>`, `<nav>`, `<main>`, `<table>`, `<dialog>`, `<form>`, `<dl>`, `<figure>`).
- **Modern Pure CSS:** Nested CSS without third-party frameworks or utility classes.
- **Responsive & RTL Compatible:** Fluid layout that adapts across desktop resolutions with full Arabic RTL support.
