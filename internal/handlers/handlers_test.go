package handlers

import (
	"archive/zip"
	"bytes"
	"embed"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"local-archive/internal/database"
)

//go:embed testdata/*
var testFS embed.FS

func setupTestApp(t *testing.T) (*App, *database.DB, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "handlers_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	uploadDir := filepath.Join(tempDir, "uploads")
	dbPath := filepath.Join(tempDir, "archive.db")
	_ = os.MkdirAll(uploadDir, 0755)

	db, err := database.Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open database: %v", err)
	}

	tmplStr := `
	{{define "login.html"}}<html><body>Login</body></html>{{end}}
	{{define "change_credentials.html"}}<html><body>Change Creds</body></html>{{end}}
	{{define "index.html"}}<html><body>Index: {{.TotalDocs}} docs</body></html>{{end}}
	{{define "view.html"}}<html><body>View: {{.Doc.Subject}}</body></html>{{end}}
	`
	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to parse templates: %v", err)
	}

	app := NewApp(db, tmpl, uploadDir, dbPath, 8080, false, testFS)
	return app, db, tempDir
}

func createAuthenticatedCookie(t *testing.T, db *database.DB) *http.Cookie {
	t.Helper()
	user, err := db.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("failed to get admin user: %v", err)
	}
	// Clear must_change_credentials for testing
	_ = db.UpdateUserCredentials(user.ID, "admin", user.PasswordHash)

	token := "test_session_token_12345"
	if err := db.CreateSession(token, user.ID, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	return &http.Cookie{
		Name:  "session_token",
		Value: token,
		Path:  "/",
	}
}

func TestHandlersFlow(t *testing.T) {
	app, db, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)
	defer db.Close()

	cookie := createAuthenticatedCookie(t, db)

	// 1. Test handleCreateDocument with Outgoing Letter
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("doc_type", "outgoing")
	_ = writer.WriteField("issue_number", "889/ص")
	_ = writer.WriteField("doc_date", "2026-09-26")
	_ = writer.WriteField("department", "محافظة بغداد")
	_ = writer.WriteField("subject", "تزويد بمعلومات")

	part, err := writer.CreateFormFile("file", "decision.pdf")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte("%PDF-1.4 official decision"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)
	w := httptest.NewRecorder()

	app.handleCreateDocument(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect StatusSeeOther (303), got %d: %s", w.Code, w.Body.String())
	}

	// 2. Verify in Database
	statsTotal, _, outgoingCount, _, err := db.GetStats()
	if err != nil || statsTotal != 1 || outgoingCount != 1 {
		t.Fatalf("expected 1 outgoing document in stats, got total=%d, outgoing=%d", statsTotal, outgoingCount)
	}

	// 3. Test handleIndex
	reqIndex := httptest.NewRequest(http.MethodGet, "/?type=outgoing", nil)
	reqIndex.AddCookie(cookie)
	wIndex := httptest.NewRecorder()
	app.handleIndex(wIndex, reqIndex)
	if wIndex.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from index, got %d", wIndex.Code)
	}
	if !strings.Contains(wIndex.Body.String(), "Index: 1 docs") {
		t.Errorf("expected body to contain 'Index: 1 docs', got: %s", wIndex.Body.String())
	}

	// 4. Test handleBackup (Zip containing archive.db and uploads)
	reqBackup := httptest.NewRequest(http.MethodGet, "/api/backup", nil)
	reqBackup.AddCookie(cookie)
	wBackup := httptest.NewRecorder()
	app.handleBackup(wBackup, reqBackup)

	if wBackup.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from backup, got %d: %s", wBackup.Code, wBackup.Body.String())
	}

	contentType := wBackup.Header().Get("Content-Type")
	if contentType != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %s", contentType)
	}

	contentDisp := wBackup.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisp, "archive_backup_") || !strings.Contains(contentDisp, ".zip") {
		t.Errorf("expected Content-Disposition to have archive_backup_*.zip, got %s", contentDisp)
	}

	// Inspect the backup zip
	zipBytes := wBackup.Body.Bytes()
	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to read returned backup zip: %v", err)
	}

	var hasDB, hasUpload bool
	for _, f := range zipReader.File {
		if f.Name == "archive.db" {
			hasDB = true
		}
		if strings.HasPrefix(f.Name, "uploads/") {
			hasUpload = true
		}
	}

	if !hasDB {
		t.Errorf("backup zip missing archive.db")
	}
	if !hasUpload {
		t.Errorf("backup zip missing uploaded files under uploads/")
	}

	// 5. Test handleRestore using the generated zip
	restoreBody := &bytes.Buffer{}
	restoreWriter := multipart.NewWriter(restoreBody)
	restorePart, err := restoreWriter.CreateFormFile("backup_file", "backup.zip")
	if err != nil {
		t.Fatalf("failed to create restore form file: %v", err)
	}
	restorePart.Write(zipBytes)
	restoreWriter.Close()

	reqRestore := httptest.NewRequest(http.MethodPost, "/api/restore", restoreBody)
	reqRestore.Header.Set("Content-Type", restoreWriter.FormDataContentType())
	reqRestore.AddCookie(cookie)
	wRestore := httptest.NewRecorder()

	app.handleRestore(wRestore, reqRestore)
	if wRestore.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 See Other after restore, got %d", wRestore.Code)
	}
	if !strings.Contains(wRestore.Header().Get("Location"), "restored=1") {
		t.Errorf("expected redirect location to contain 'restored=1', got '%s'", wRestore.Header().Get("Location"))
	}
}
