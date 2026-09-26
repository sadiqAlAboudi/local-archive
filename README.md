# Local Archive 📁 (الأرشيف المحلي)

A clean, minimalist, offline local web application for archiving official incoming and outgoing documents, letters, and administrative records (PDFs and images) with SQLite storage, quick search, retrieval, printing, and automated full archive backups and restoration (database + files).

---

## 🚀 Quick Start (Development / Linux)

Run the server with:

```bash
go run main.go
```

Then open your browser to **[http://localhost:8080](http://localhost:8080)**.

### Initial Credentials
- **Username:** `admin`
- **Password:** `admin`

> **First Login Notice:** For security, the system will prompt you immediately after your first login to set your own custom username and password before accessing the archive.

---

## 📋 Document Types & Metadata Fields

Local Archive supports archiving both **Incoming Documents (الكتب الواردة)** and **Outgoing Documents (الكتب الصادرة)**:

### 1. الكتب الواردة (Incoming Documents)
- **نوع الوثيقة:** كتاب وارد
- **رقم التسلسل** (Serial Number)
- **التاريخ** (تاريخ تسجيل الوارد)
- **اسم الدائرة** (الجهة الوارد منها الكتاب)
- **رقم الكتاب** (رقم كتاب الجهة الصادر منها)
- **تاريخ الكتاب** (تاريخ كتاب الجهة الصادر منها)
- **الموضوع** (Subject)
- **الملف المرفق** (Attached PDF document or Image)
- **تسمية التحميل التلقائية:** `[رقم التسلسل]-[الموضوع].[extension]`

### 2. الكتب الصادرة (Outgoing Documents)
- **نوع الوثيقة:** كتاب صادر
- **العدد** (Issue / Letter Number)
- **التاريخ** (تاريخ صدور الكتاب)
- **الجهة الصادر إليها** (اسم الدائرة أو الجهة المستلمة)
- **الموضوع** (Subject)
- **الملف المرفق** (Attached PDF document or Image)
- **تسمية التحميل التلقائية:** `[العدد]-[الموضوع].[extension]`

---

## 🔍 Multi-Field Search & Filtering

- **أزرار التصفية السريعة (Quick Filter Tabs):** تصفية فورية لعرض "جميع الوثائق"، "الكتب الواردة فقط"، أو "الكتب الصادرة فقط".
- **البحث السريع المتعدد (Quick Multi-Word Search):** ابحث بكلمة أو عدة كلمات (مثال: `التربية 2026` أو `742 نقل`) للبحث الفوري عبر كافة الحقول في آن واحد.
- **التصفية المخصصة (Custom Field Filtering):** انقر على **"تصفية مخصصة حسب عدة حقول"** للتصفية بحسب رقم التسلسل أو العدد، اسم الدائرة، رقم الكتاب، التاريخ، أو الموضوع.

---

## 💾 Data Folder, Backup & Restore (القاعدة والملفات)

- **المجلدات التلقائية:** عند تشغيل التطبيق، يتم تلقائياً إنشاء مجلد `data/` ومجلد المرفقات `data/uploads/` وقاعدة بيانات SQLite في `data/archive.db`.
- **النسخ الاحتياطي الشامل (Full Backup):**
  - عند النقر على زر **"نسخ احتياطي شامل"** في الشريط العلوي، يقوم النظام بإنشاء ملف مضغوط بصيغة ZIP (`archive_backup_*.zip`) يحتوي على:
    1. نسخة سليمة ومحدثة من قاعدة البيانات (`archive.db`).
    2. كافة الملفات والوثائق المرفقة داخل مجلد `uploads/`.
- **استعادة النسخة الاحتياطية (Restore Backup):**
  - انقر على زر **"استعادة نسخة احتياطية"** في الشريط العلوي واختر ملف النسخة الاحتياطية المضغوط (`.zip` أو ملف `.db`).
  - يتحقق النظام تلقائياً من سلامة قاعدة البيانات والملفات ويقوم باستبدال الأرشيف وإعادة تحميله بشكل آمن ومباشر.

---

## 💻 PWA Desktop App Installation

Local Archive is fully configured as a Progressive Web App (PWA):
- Open **http://localhost:8080** in Google Chrome, Microsoft Edge, or Brave.
- Click the **Install** button in the browser address bar or choose **"Install Local Archive"** / **"تثبيت التطبيق"** from the browser menu.
- The app will run in its own clean, dedicated desktop window with its custom archive icon.

---

## 🪟 Windows Installer (مثبت ويندوز الذاتي)

يتوفر مثبت تنفيذي جاهز لنظام ويندوز (`LocalArchive-Setup.exe`) دون الحاجة لتثبيت Go أو أي برامج وسيطة أو تشغيل برمجيات نصية (Scripts).

### التثبيت بنقرة واحدة:
1. قم بتشغيل ملف **`LocalArchive-Setup.exe`**.
2. يقوم المثبت تلقائياً بـ:
   - تثبيت التطبيق في مجلد البرامج الخاص بالمستخدم (`%LOCALAPPDATA%\LocalArchive`).
   - إنشاء اختصار رسمي على سطح المكتب وفي قائمة ابدأ مع الأيقونة المخصصة.
   - تهيئة التشغيل التلقائي مع إقلاع نظام ويندوز في الخلفية عبر سجل النظام (Registry).
   - تسجيل الأرشيف المحلي في قائمة البرامج المثبتة في ويندوز (إضافة وإزالة البرامج) مع إمكانية إلغاء التثبيت النظيف في أي وقت.
   - تشغيل التطبيق وفتح المتصفح فوراً على `http://localhost:8080`.

### إعادة بناء المثبت لنظام ويندوز (للمطورين):
```bash
./build-windows.sh
```
أو عبر أمر Go المباشر:
```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o archive.exe .
mkdir -p cmd/installer/payload
cp archive.exe cmd/installer/payload/archive.exe
cp static/favicon.ico cmd/installer/payload/favicon.ico
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o LocalArchive-Setup.exe ./cmd/installer
```

---

## 🏗️ Project Architecture

```
local-archive/
├── main.go                       # Minimal application entry point and flag parsing
├── build-windows.sh              # Windows binary and installer build script
├── templates/                    # HTML templates (embedded)
├── static/                       # Static assets: CSS, JS, PWA icons (embedded)
├── cmd/
│   └── installer/                # Standalone Windows native installer (LocalArchive-Setup.exe)
├── installer/
│   └── local-archive.iss         # Inno Setup installer script
├── internal/
│   ├── models/                   # Document, User, Session, and View data models
│   ├── database/                 # Pure-Go SQLite migrations and queries
│   ├── backup/                   # Backup & restore engine (zip & db validation)
│   ├── sysutil/                  # System helpers, paths, and platform helpers
│   └── handlers/                 # HTTP controllers, session auth, and routing
└── data/                         # Local storage (created automatically)
    ├── archive.db                # SQLite database (WAL mode)
    └── uploads/                  # Uploaded document files
```

---

## 🎨 Design & Rules Compliance

- **Arabic Interface (RTL):** Fully designed in Arabic with right-to-left layout.
- **Minimalist Black Theme:** Clean `#000000` background with neutral dark surfaces and high-contrast typography.
- **Custom Scrollbar Styling:** Refined dark scrollbars using standard `scrollbar-width` and `scrollbar-color` with cross-browser WebKit support.
- **Semantic HTML Only:** Built using native tags (`<header>`, `<nav>`, `<main>`, `<table>`, `<dialog>`, `<form>`, `<dl>`, `<figure>`).
- **Nested CSS & Zero Custom Classes:** Pure CSS nesting without a single `class="..."` anywhere in the codebase.
