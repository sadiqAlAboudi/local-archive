package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"local-archive/internal/database"
	"local-archive/internal/sysutil"
)

func setupTestApp(t *testing.T) (*App, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "wails_app_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	paths := sysutil.AppDataPaths{
		BaseDir:   tempDir,
		DBPath:    filepath.Join(tempDir, "archive.db"),
		UploadDir: filepath.Join(tempDir, "uploads"),
	}
	_ = os.MkdirAll(paths.UploadDir, 0755)

	db, err := database.Open(paths.DBPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open database: %v", err)
	}

	app := NewApp(db, paths)
	return app, tempDir
}

func TestWailsAppCRUD(t *testing.T) {
	app, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)
	defer app.shutdown(context.TODO())

	// Create dummy test file
	dummyFilePath := filepath.Join(tempDir, "test_doc.pdf")
	if err := os.WriteFile(dummyFilePath, []byte("%PDF-1.4 official letter"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Test CreateDocument (Incoming)
	incomingDoc, err := app.CreateDocument(CreateDocumentInput{
		DocType:      "incoming",
		SerialNumber: "105/و",
		DocDate:      "2026-09-27",
		Department:   "وزارة المالية",
		LetterNumber: "991",
		LetterDate:   "2026-09-25",
		Subject:      "تخصيصات مالية",
		SourcePath:   dummyFilePath,
	})
	if err != nil {
		t.Fatalf("CreateDocument incoming failed: %v", err)
	}
	if incomingDoc.ID == 0 {
		t.Fatalf("expected non-zero ID for created document")
	}

	// 2. Test CreateDocument (Outgoing)
	outgoingDoc, err := app.CreateDocument(CreateDocumentInput{
		DocType:     "outgoing",
		IssueNumber: "442/ص",
		DocDate:     "2026-09-27",
		Department:  "محافظة بغداد",
		Subject:     "إشعار رسمي",
		SourcePath:  dummyFilePath,
	})
	if err != nil {
		t.Fatalf("CreateDocument outgoing failed: %v", err)
	}

	// 3. Test GetStats
	stats, err := app.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalDocs != 2 || stats.TotalIncoming != 1 || stats.TotalOutgoing != 1 {
		t.Fatalf("unexpected stats: total=%d, incoming=%d, outgoing=%d", stats.TotalDocs, stats.TotalIncoming, stats.TotalOutgoing)
	}
	if stats.ShowBackupReminder {
		t.Fatalf("expected ShowBackupReminder to be false for a fresh archive, got true")
	}

	// 4. Test GetDocuments with filters
	docsIncoming, err := app.GetDocuments("", "incoming")
	if err != nil || len(docsIncoming) != 1 {
		t.Fatalf("expected 1 incoming doc, got %d (err: %v)", len(docsIncoming), err)
	}

	docsSearch, err := app.GetDocuments("تخصيصات", "all")
	if err != nil || len(docsSearch) != 1 {
		t.Fatalf("expected 1 doc from query, got %d (err: %v)", len(docsSearch), err)
	}

	// 5. Test UpdateDocument
	err = app.UpdateDocument(UpdateDocumentInput{
		ID:           incomingDoc.ID,
		DocType:      "incoming",
		SerialNumber: "105/معدل",
		DocDate:      "2026-09-27",
		Department:   "وزارة المالية",
		LetterNumber: "991",
		LetterDate:   "2026-09-25",
		Subject:      "تخصيصات مالية معدلة",
	})
	if err != nil {
		t.Fatalf("UpdateDocument failed: %v", err)
	}

	updated, err := app.GetDocument(incomingDoc.ID)
	if err != nil || updated.SerialNumber != "105/معدل" || updated.Subject != "تخصيصات مالية معدلة" {
		t.Fatalf("document was not updated correctly: %+v", updated)
	}

	// 6. Test DeleteDocument
	err = app.DeleteDocument(outgoingDoc.ID)
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	statsAfterDelete, _ := app.GetStats()
	if statsAfterDelete.TotalDocs != 1 {
		t.Fatalf("expected 1 doc after deletion, got %d", statsAfterDelete.TotalDocs)
	}

	// 7. Test Backup Reminder when 6 days pass
	sixDaysAgo := time.Now().Add(-6 * 24 * time.Hour)
	if err := app.db.SetLastBackupTime(sixDaysAgo); err != nil {
		t.Fatalf("SetLastBackupTime failed: %v", err)
	}
	statsAfter6Days, _ := app.GetStats()
	if !statsAfter6Days.ShowBackupReminder || statsAfter6Days.LastBackupDays < 5 {
		t.Fatalf("expected ShowBackupReminder to be true after 6 days, got %+v", statsAfter6Days)
	}
}
