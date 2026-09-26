package handlers

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"local-archive/internal/database"
	"local-archive/internal/sysutil"
)

// App manages web application state and HTTP handlers.
type App struct {
	db          *database.DB
	tmpl        *template.Template
	uploadDir   string
	dbPath      string
	backupMutex sync.Mutex
	port        int
	openBrowser bool
	fs          embed.FS
}

// NewApp creates and initializes a new App instance.
func NewApp(db *database.DB, tmpl *template.Template, uploadDir, dbPath string, port int, openBrowser bool, fs embed.FS) *App {
	return &App{
		db:          db,
		tmpl:        tmpl,
		uploadDir:   uploadDir,
		dbPath:      dbPath,
		port:        port,
		openBrowser: openBrowser,
		fs:          fs,
	}
}

// RegisterRoutes registers all HTTP endpoints on the provided ServeMux.
func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Embedded static files handler
	staticServer := http.FileServer(http.FS(a.fs))
	mux.Handle("/static/", staticServer)

	// PWA and Favicon endpoints
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		icoData, err := a.fs.ReadFile("static/favicon.ico")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write(icoData)
	})

	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := a.fs.ReadFile("static/manifest.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Write(data)
	})

	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := a.fs.ReadFile("static/sw.js")
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
	mux.HandleFunc("/api/restore", a.requireAuth(a.handleRestore, true))
}

// requireAuth ensures the request has an active valid session.
func (a *App) requireAuth(next http.HandlerFunc, enforceCredentialChange bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, expiresAt, mustChange, err := a.db.GetSessionUser(cookie.Value)
		if err != nil || time.Now().After(expiresAt) || userID == 0 {
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
		if enforceCredentialChange && mustChange {
			http.Redirect(w, r, "/change-credentials", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}

// renderTemplate executes an embedded HTML template.
func (a *App) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Template error in %s: %v", name, err)
		http.Error(w, "خطأ في عرض الصفحة", http.StatusInternalServerError)
	}
}

// Start starts listening on the configured port.
func (a *App) Start() error {
	mux := http.NewServeMux()
	a.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", a.port)
	url := fmt.Sprintf("http://localhost:%d", a.port)

	log.Println("==================================================")
	log.Printf(" Local Archive is running at: %s", url)
	log.Printf(" Database location: %s", a.dbPath)
	if a.db.HasDefaultCredentials() {
		log.Println(" Default Login: username 'admin', password 'admin'")
		log.Println(" Password change is required upon first login.")
	}
	log.Println(" Press Ctrl+C to stop the server")
	log.Println("==================================================")

	if a.openBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			sysutil.OpenBrowserURL(url)
		}()
	}

	return http.ListenAndServe(addr, mux)
}
