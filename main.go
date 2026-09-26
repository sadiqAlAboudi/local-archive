package main

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

type Document struct {
	ID               int64
	SerialNumber     string // رقم التسلسل
	DocDate          string // التاريخ
	Department       string // اسم الدائرة
	LetterNumber     string // رقم الكتاب
	LetterDate       string // تاريخ الكتاب
	Subject          string // الموضوع
	Filename         string
	OriginalFilename string
	FileType         string // "pdf" or "image"
	MimeType         string
	FileSize         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (d Document) FormattedSize() string {
	return formatBytes(d.FileSize)
}

func (d Document) FormattedCreatedAt() string {
	return d.CreatedAt.Format("2006-01-02")
}

type IndexViewData struct {
	TotalDocs     int64
	StorageUsed   string
	Query         string
	FilterSerial  string
	FilterDept    string
	FilterLetter  string
	FilterDate    string
	FilterSubject string
	HasFilter     bool
	Documents     []Document
}

type ViewDocData struct {
	Doc Document
}

type LoginViewData struct {
	Error             string
	ShowDefaultNotice bool
}

func (a *App) hasDefaultCredentials() bool {
	var count int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM users WHERE must_change_credentials = 1").Scan(&count)
	return count > 0
}

type ChangeCredsViewData struct {
	Error           string
	CurrentUsername string
}

type App struct {
	db          *sql.DB
	tmpl        *template.Template
	uploadDir   string
	dbPath      string
	mutex       sync.RWMutex
	port        int
	openBrowser bool
}

func main() {
	portFlag := flag.Int("port", 8080, "Port for web server")
	noBrowserFlag := flag.Bool("no-browser", false, "Disable automatically opening web browser")
	installFlag := flag.Bool("install", false, "Install to Windows Startup folder for automatic boot launch")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall from Windows Startup folder")
	flag.Parse()

	// Handle Windows startup setup/removal if requested
	if *installFlag {
		if err := configureWindowsStartup(true); err != nil {
			fmt.Printf("Error installing startup task: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Local Archive has been installed to start automatically on Windows boot.")
		return
	}
	if *uninstallFlag {
		if err := configureWindowsStartup(false); err != nil {
			fmt.Printf("Error removing startup task: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Local Archive automatic startup has been removed.")
		return
	}

	app, err := newApp(*portFlag, !*noBrowserFlag)
	if err != nil {
		log.Fatalf("Failed to initialize Local Archive: %v", err)
	}

	app.start()
}

func getDataDir() string {
	// If running as a standalone compiled binary (not via "go run" in temp dir)
	if exePath, err := os.Executable(); err == nil {
		clean := filepath.Clean(exePath)
		// "go run" runs binaries from temp build dirs containing "go-build" or os.TempDir
		if !strings.Contains(clean, "go-build") && !strings.HasPrefix(clean, os.TempDir()) {
			return filepath.Join(filepath.Dir(clean), "data")
		}
	}

	// When running via "go run", use the current working directory
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, "data")
	}

	return filepath.Join(".", "data")
}

func newApp(port int, openBrowser bool) (*App, error) {
	dataDir := getDataDir()
	uploadDir := filepath.Join(dataDir, "uploads")
	dbPath := filepath.Join(dataDir, "archive.db")

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create data directory at %s: %w", dataDir, err)
	}

	// Connect to SQLite (creates archive.db automatically if it doesn't exist)
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Parse embedded templates
	tmpl, err := template.ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	app := &App{
		db:          db,
		tmpl:        tmpl,
		uploadDir:   uploadDir,
		dbPath:      dbPath,
		port:        port,
		openBrowser: openBrowser,
	}

	if err := app.initDB(); err != nil {
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	return app, nil
}

func (a *App) initDB() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			must_change_credentials INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			serial_number TEXT NOT NULL,
			doc_date TEXT NOT NULL,
			department TEXT NOT NULL,
			letter_number TEXT NOT NULL,
			letter_date TEXT NOT NULL,
			subject TEXT NOT NULL,
			filename TEXT NOT NULL,
			original_filename TEXT NOT NULL,
			file_type TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_serial ON documents(serial_number);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_dept ON documents(department);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_letterno ON documents(letter_number);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_subject ON documents(subject);`,
	}

	for _, q := range queries {
		if _, err := a.db.Exec(q); err != nil {
			return err
		}
	}

	// Migrate users table if column must_change_credentials was missing in existing DB
	_, _ = a.db.Exec("ALTER TABLE users ADD COLUMN must_change_credentials INTEGER DEFAULT 1;")

	// Seed default admin user if none exists
	var count int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}

	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = a.db.Exec("INSERT INTO users (username, password_hash, must_change_credentials) VALUES (?, ?, 1)", "admin", string(hash))
		if err != nil {
			return err
		}
		log.Println("Initialized default administrator account (username: admin, password: admin)")
	}

	return nil
}

func (a *App) start() {
	mux := http.NewServeMux()

	// Embedded static files handler
	staticServer := http.FileServer(http.FS(embeddedFS))
	mux.Handle("/static/", staticServer)

	// PWA and Favicon endpoints
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		icoData, err := embeddedFS.ReadFile("static/favicon.ico")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write(icoData)
	})

	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFS.ReadFile("static/manifest.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Write(data)
	})

	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFS.ReadFile("static/sw.js")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Write(data)
	})

	// Public routes
	mux.HandleFunc("/login", a.handleLogin)
	mux.HandleFunc("/logout", a.handleLogout)

	// Protected routes
	mux.HandleFunc("/change-credentials", a.requireAuth(a.handleChangeCredentials, false))
	mux.HandleFunc("/", a.requireAuth(a.handleIndex, true))
	mux.HandleFunc("/view", a.requireAuth(a.handleView, true))
	mux.HandleFunc("/api/file", a.requireAuth(a.handleServeFile, true))
	mux.HandleFunc("/api/download", a.requireAuth(a.handleDownloadFile, true))
	mux.HandleFunc("/api/documents", a.requireAuth(a.handleCreateDocument, true))
	mux.HandleFunc("/api/documents/edit", a.requireAuth(a.handleEditDocument, true))
	mux.HandleFunc("/api/documents/delete", a.requireAuth(a.handleDeleteDocument, true))
	mux.HandleFunc("/api/backup", a.requireAuth(a.handleBackup, true))

	addr := fmt.Sprintf(":%d", a.port)
	url := fmt.Sprintf("http://localhost:%d", a.port)

	log.Println("==================================================")
	log.Printf(" Local Archive is running at: %s", url)
	log.Printf(" Database location: %s", a.dbPath)
	if a.hasDefaultCredentials() {
		log.Println(" Default Login: username 'admin', password 'admin'")
		log.Println(" Password change is required upon first login.")
	}
	log.Println(" Press Ctrl+C to stop the server")
	log.Println("==================================================")

	// Auto-open browser
	if a.openBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowserURL(url)
		}()
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Authentication Middleware with first-login password change enforcement
func (a *App) requireAuth(next http.HandlerFunc, enforceCredentialChange bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var userID int64
		var expiresAt time.Time
		var mustChange int
		err = a.db.QueryRow(`
			SELECT s.user_id, s.expires_at, u.must_change_credentials 
			FROM sessions s 
			JOIN users u ON s.user_id = u.id 
			WHERE s.token = ?
		`, cookie.Value).Scan(&userID, &expiresAt, &mustChange)

		if err != nil || time.Now().After(expiresAt) {
			http.SetCookie(w, &http.Cookie{
				Name:     "session_token",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
			})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Enforce changing username & password after first login
		if enforceCredentialChange && mustChange == 1 {
			http.Redirect(w, r, "/change-credentials", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// If already logged in, check if credential change is needed
		if cookie, err := r.Cookie("session_token"); err == nil && cookie.Value != "" {
			var uid int64
			var exp time.Time
			var mustChange int
			if err := a.db.QueryRow(`
				SELECT s.user_id, s.expires_at, u.must_change_credentials 
				FROM sessions s 
				JOIN users u ON s.user_id = u.id 
				WHERE s.token = ?
			`, cookie.Value).Scan(&uid, &exp, &mustChange); err == nil && time.Now().Before(exp) {
				if mustChange == 1 {
					http.Redirect(w, r, "/change-credentials", http.StatusSeeOther)
					return
				}
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		a.renderTemplate(w, "login.html", LoginViewData{
			ShowDefaultNotice: a.hasDefaultCredentials(),
		})
		return
	}

	if r.Method == http.MethodPost {
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		var userID int64
		var passwordHash string
		var mustChange int
		err := a.db.QueryRow("SELECT id, password_hash, must_change_credentials FROM users WHERE username = ?", username).Scan(&userID, &passwordHash, &mustChange)
		if err != nil {
			a.renderTemplate(w, "login.html", LoginViewData{
				Error:             "اسم المستخدم أو كلمة المرور غير صحيحة",
				ShowDefaultNotice: a.hasDefaultCredentials(),
			})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
			a.renderTemplate(w, "login.html", LoginViewData{
				Error:             "اسم المستخدم أو كلمة المرور غير صحيحة",
				ShowDefaultNotice: a.hasDefaultCredentials(),
			})
			return
		}

		tokenBytes := make([]byte, 32)
		rand.Read(tokenBytes)
		token := hex.EncodeToString(tokenBytes)
		expiresAt := time.Now().Add(30 * 24 * time.Hour)

		_, err = a.db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
		if err != nil {
			http.Error(w, "فشل إنشاء الجلسة", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			Expires:  expiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		if mustChange == 1 {
			http.Redirect(w, r, "/change-credentials", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (a *App) handleChangeCredentials(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("session_token")
	var userID int64
	var currentUsername string
	err := a.db.QueryRow(`
		SELECT u.id, u.username 
		FROM users u 
		JOIN sessions s ON u.id = s.user_id 
		WHERE s.token = ?
	`, cookie.Value).Scan(&userID, &currentUsername)

	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		a.renderTemplate(w, "change_credentials.html", ChangeCredsViewData{
			CurrentUsername: currentUsername,
		})
		return
	}

	if r.Method == http.MethodPost {
		newUsername := strings.TrimSpace(r.FormValue("username"))
		newPassword := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if newUsername == "" || newPassword == "" {
			a.renderTemplate(w, "change_credentials.html", ChangeCredsViewData{
				CurrentUsername: currentUsername,
				Error:           "يجب إدخال اسم المستخدم وكلمة المرور الجديدة",
			})
			return
		}

		if newPassword != confirmPassword {
			a.renderTemplate(w, "change_credentials.html", ChangeCredsViewData{
				CurrentUsername: newUsername,
				Error:           "كلمة المرور وتأكيد كلمة المرور غير متطابقين",
			})
			return
		}

		if len(newPassword) < 4 {
			a.renderTemplate(w, "change_credentials.html", ChangeCredsViewData{
				CurrentUsername: newUsername,
				Error:           "يجب أن تتكون كلمة المرور من 4 خانات على الأقل",
			})
			return
		}

		// Check if username taken by another user
		var existsID int64
		err = a.db.QueryRow("SELECT id FROM users WHERE username = ? AND id != ?", newUsername, userID).Scan(&existsID)
		if err == nil {
			a.renderTemplate(w, "change_credentials.html", ChangeCredsViewData{
				CurrentUsername: newUsername,
				Error:           "اسم المستخدم هذا مستخدم بالفعل، اختر اسماً آخر",
			})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "فشل تشفير كلمة المرور", http.StatusInternalServerError)
			return
		}

		_, err = a.db.Exec(`
			UPDATE users 
			SET username = ?, password_hash = ?, must_change_credentials = 0 
			WHERE id = ?
		`, newUsername, string(hash), userID)

		if err != nil {
			a.renderTemplate(w, "change_credentials.html", ChangeCredsViewData{
				CurrentUsername: newUsername,
				Error:           "تعذر حفظ التغييرات: " + err.Error(),
			})
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session_token"); err == nil && cookie.Value != "" {
		a.db.Exec("DELETE FROM sessions WHERE token = ?", cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	filterSerial := strings.TrimSpace(r.URL.Query().Get("serial"))
	filterDept := strings.TrimSpace(r.URL.Query().Get("dept"))
	filterLetter := strings.TrimSpace(r.URL.Query().Get("letterno"))
	filterDate := strings.TrimSpace(r.URL.Query().Get("date"))
	filterSubject := strings.TrimSpace(r.URL.Query().Get("subject"))

	hasFilter := searchQuery != "" || filterSerial != "" || filterDept != "" || filterLetter != "" || filterDate != "" || filterSubject != ""

	// Retrieve statistics
	var totalDocs int64
	var totalBytes sql.NullInt64

	a.db.QueryRow("SELECT COUNT(*), SUM(file_size) FROM documents").Scan(&totalDocs, &totalBytes)

	storageFormatted := formatBytes(totalBytes.Int64)

	// Build dynamic search conditions
	var conditions []string
	var args []interface{}

	if searchQuery != "" {
		// Multi-word search across all fields
		words := strings.Fields(searchQuery)
		for _, w := range words {
			conditions = append(conditions, "(serial_number LIKE ? OR doc_date LIKE ? OR department LIKE ? OR letter_number LIKE ? OR letter_date LIKE ? OR subject LIKE ?)")
			pattern := "%" + w + "%"
			args = append(args, pattern, pattern, pattern, pattern, pattern, pattern)
		}
	}

	if filterSerial != "" {
		conditions = append(conditions, "serial_number LIKE ?")
		args = append(args, "%"+filterSerial+"%")
	}
	if filterDept != "" {
		conditions = append(conditions, "department LIKE ?")
		args = append(args, "%"+filterDept+"%")
	}
	if filterLetter != "" {
		conditions = append(conditions, "letter_number LIKE ?")
		args = append(args, "%"+filterLetter+"%")
	}
	if filterDate != "" {
		conditions = append(conditions, "(doc_date LIKE ? OR letter_date LIKE ?)")
		args = append(args, "%"+filterDate+"%", "%"+filterDate+"%")
	}
	if filterSubject != "" {
		conditions = append(conditions, "subject LIKE ?")
		args = append(args, "%"+filterSubject+"%")
	}

	querySQL := `
		SELECT id, serial_number, doc_date, department, letter_number, letter_date, subject, filename, original_filename, file_type, mime_type, file_size, created_at, updated_at
		FROM documents
	`
	if len(conditions) > 0 {
		querySQL += " WHERE " + strings.Join(conditions, " AND ")
	}
	querySQL += " ORDER BY id DESC"

	rows, err := a.db.Query(querySQL, args...)
	if err != nil {
		http.Error(w, "خطأ في استعلام الوثائق: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		var createdAtStr, updatedAtStr string
		err := rows.Scan(
			&d.ID, &d.SerialNumber, &d.DocDate, &d.Department,
			&d.LetterNumber, &d.LetterDate, &d.Subject,
			&d.Filename, &d.OriginalFilename, &d.FileType,
			&d.MimeType, &d.FileSize, &createdAtStr, &updatedAtStr,
		)
		if err != nil {
			continue
		}
		d.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		if d.CreatedAt.IsZero() {
			d.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating document rows: %v", err)
	}

	data := IndexViewData{
		TotalDocs:     totalDocs,
		StorageUsed:   storageFormatted,
		Query:         searchQuery,
		FilterSerial:  filterSerial,
		FilterDept:    filterDept,
		FilterLetter:  filterLetter,
		FilterDate:    filterDate,
		FilterSubject: filterSubject,
		HasFilter:     hasFilter,
		Documents:     docs,
	}

	a.renderTemplate(w, "index.html", data)
}

func (a *App) handleView(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var d Document
	var createdAtStr, updatedAtStr string
	err = a.db.QueryRow(`
		SELECT id, serial_number, doc_date, department, letter_number, letter_date, subject, filename, original_filename, file_type, mime_type, file_size, created_at, updated_at
		FROM documents WHERE id = ?
	`, id).Scan(
		&d.ID, &d.SerialNumber, &d.DocDate, &d.Department,
		&d.LetterNumber, &d.LetterDate, &d.Subject,
		&d.Filename, &d.OriginalFilename, &d.FileType,
		&d.MimeType, &d.FileSize, &createdAtStr, &updatedAtStr,
	)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	d.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	if d.CreatedAt.IsZero() {
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	}

	a.renderTemplate(w, "view.html", ViewDocData{Doc: d})
}

func (a *App) handleServeFile(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var filename, mimeType string
	err = a.db.QueryRow("SELECT filename, mime_type FROM documents WHERE id = ?", id).Scan(&filename, &mimeType)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(a.uploadDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, filePath)
}

func sanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\r", "\n", "\t"}
	for _, inv := range invalid {
		name = strings.ReplaceAll(name, inv, "-")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "وثيقة"
	}
	return name
}

func (a *App) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var filename, originalFilename, mimeType, serialNumber, subject string
	err = a.db.QueryRow("SELECT filename, original_filename, mime_type, serial_number, subject FROM documents WHERE id = ?", id).Scan(&filename, &originalFilename, &mimeType, &serialNumber, &subject)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(a.uploadDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	ext := filepath.Ext(originalFilename)
	if ext == "" {
		ext = filepath.Ext(filename)
	}
	downloadName := sanitizeFilename(fmt.Sprintf("%s-%s", serialNumber, subject)) + ext
	encodedName := url.PathEscape(downloadName)

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, downloadName, encodedName))
	http.ServeFile(w, r, filePath)
}

func (a *App) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 50 MB limit
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "تجاوز حجم الملف الحد المسموح أو تعذر قراءة النموذج", http.StatusBadRequest)
		return
	}

	serialNumber := strings.TrimSpace(r.FormValue("serial_number"))
	docDate := strings.TrimSpace(r.FormValue("doc_date"))
	department := strings.TrimSpace(r.FormValue("department"))
	letterNumber := strings.TrimSpace(r.FormValue("letter_number"))
	letterDate := strings.TrimSpace(r.FormValue("letter_date"))
	subject := strings.TrimSpace(r.FormValue("subject"))

	if serialNumber == "" || subject == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "الملف المرفق مطلوب", http.StatusBadRequest)
		return
	}
	defer file.Close()

	originalFilename := header.Filename
	ext := strings.ToLower(filepath.Ext(originalFilename))

	var fileType string
	var mimeType string

	switch ext {
	case ".pdf":
		fileType = "pdf"
		mimeType = "application/pdf"
	case ".jpg", ".jpeg":
		fileType = "image"
		mimeType = "image/jpeg"
	case ".png":
		fileType = "image"
		mimeType = "image/png"
	case ".webp":
		fileType = "image"
		mimeType = "image/webp"
	case ".gif":
		fileType = "image"
		mimeType = "image/gif"
	default:
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		file.Seek(0, io.SeekStart)
		detected := http.DetectContentType(buf[:n])
		if strings.HasPrefix(detected, "image/") {
			fileType = "image"
			mimeType = detected
		} else if detected == "application/pdf" {
			fileType = "pdf"
			mimeType = detected
		} else {
			http.Error(w, "صيغة الملف غير مدعومة. يرجى رفع ملف PDF أو صورة.", http.StatusBadRequest)
			return
		}
	}

	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	storageName := fmt.Sprintf("%d_%s%s", time.Now().Unix(), hex.EncodeToString(randBytes), ext)
	dstPath := filepath.Join(a.uploadDir, storageName)

	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "فشل حفظ الملف المرفق", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	writtenBytes, err := io.Copy(dst, file)
	if err != nil {
		http.Error(w, "فشل كتابة محتوى الملف", http.StatusInternalServerError)
		return
	}

	// Insert into DB with the 6 fields
	_, err = a.db.Exec(`
		INSERT INTO documents (serial_number, doc_date, department, letter_number, letter_date, subject, filename, original_filename, file_type, mime_type, file_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, serialNumber, docDate, department, letterNumber, letterDate, subject, storageName, originalFilename, fileType, mimeType, writtenBytes)

	if err != nil {
		os.Remove(dstPath)
		http.Error(w, "فشل تسجيل بيانات الوثيقة: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleEditDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	serialNumber := strings.TrimSpace(r.FormValue("serial_number"))
	docDate := strings.TrimSpace(r.FormValue("doc_date"))
	department := strings.TrimSpace(r.FormValue("department"))
	letterNumber := strings.TrimSpace(r.FormValue("letter_number"))
	letterDate := strings.TrimSpace(r.FormValue("letter_date"))
	subject := strings.TrimSpace(r.FormValue("subject"))

	if serialNumber != "" && subject != "" {
		a.db.Exec(`
			UPDATE documents 
			SET serial_number = ?, doc_date = ?, department = ?, letter_number = ?, letter_date = ?, subject = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, serialNumber, docDate, department, letterNumber, letterDate, subject, id)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var filename string
	err = a.db.QueryRow("SELECT filename FROM documents WHERE id = ?", id).Scan(&filename)
	if err == nil {
		filePath := filepath.Join(a.uploadDir, filename)
		os.Remove(filePath)
	}

	a.db.Exec("DELETE FROM documents WHERE id = ?", id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleBackup(w http.ResponseWriter, r *http.Request) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	timestamp := time.Now().Format("2006-01-02_150405")
	tempBackupPath := filepath.Join(os.TempDir(), fmt.Sprintf("archive_backup_%s.db", timestamp))
	defer os.Remove(tempBackupPath)

	_, err := a.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", filepath.ToSlash(tempBackupPath)))
	if err != nil {
		src, srcErr := os.Open(a.dbPath)
		if srcErr != nil {
			http.Error(w, "تعذر إنشاء النسخة الاحتياطية", http.StatusInternalServerError)
			return
		}
		defer src.Close()

		dst, dstErr := os.Create(tempBackupPath)
		if dstErr != nil {
			http.Error(w, "تعذر كتابة ملف النسخة الاحتياطية", http.StatusInternalServerError)
			return
		}
		io.Copy(dst, src)
		dst.Close()
	}

	backupFile, err := os.Open(tempBackupPath)
	if err != nil {
		http.Error(w, "تعذر قراءة ملف النسخة الاحتياطية", http.StatusInternalServerError)
		return
	}
	defer backupFile.Close()

	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="archive_backup_%s.db"`, timestamp))
	io.Copy(w, backupFile)
}

func (a *App) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Template error in %s: %v", name, err)
		http.Error(w, "خطأ في عرض الصفحة", http.StatusInternalServerError)
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d بايت", b)
	}
	div, exp := int64(unit), 0
	units := []string{"كيلوبايت", "ميجابايت", "جيجابايت", "تيرابايت"}
	for n := b / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

func openBrowserURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func configureWindowsStartup(enable bool) error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return fmt.Errorf("APPDATA environment variable not found")
	}

	startupDir := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	vbsPath := filepath.Join(startupDir, "LocalArchive.vbs")

	if !enable {
		return os.Remove(vbsPath)
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	vbsContent := fmt.Sprintf(`Set WshShell = CreateObject("WScript.Shell")
WshShell.CurrentDirectory = "%s"
WshShell.Run """%s"" -no-browser", 0, False
`, filepath.Dir(exePath), exePath)

	return os.WriteFile(vbsPath, []byte(vbsContent), 0644)
}
