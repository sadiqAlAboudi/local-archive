package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"local-archive/internal/models"
)

func setupTestDB(t *testing.T) (*DB, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "archive_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test_archive.db")
	db, err := Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open test db: %v", err)
	}

	return db, tempDir
}

func TestIncomingAndOutgoingDocuments(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer db.Close()

	// 1. Create Incoming Document
	incomingDoc := models.Document{
		DocType:          "incoming",
		SerialNumber:     "101",
		DocDate:          "2026-09-24",
		Department:       "وزارة التربية",
		LetterNumber:     "4589/ب",
		LetterDate:       "2026-09-20",
		Subject:          "طلب تثبيت موظفين",
		Filename:         "test_in.pdf",
		OriginalFilename: "document.pdf",
		FileType:         "pdf",
		MimeType:         "application/pdf",
		FileSize:         1024,
	}

	if err := db.CreateDocument(&incomingDoc); err != nil {
		t.Fatalf("failed to create incoming doc: %v", err)
	}

	if incomingDoc.ID == 0 {
		t.Fatalf("expected document ID to be set, got 0")
	}

	// 2. Create Outgoing Document (has issue_number, no serial_number or letter_number)
	outgoingDoc := models.Document{
		DocType:          "outgoing",
		IssueNumber:      "742/ص",
		DocDate:          "2026-09-25",
		Department:       "وزارة التعليم العالي",
		Subject:          "إيفاد رسمي لدراسة الماجستير",
		Filename:         "test_out.jpg",
		OriginalFilename: "letter.jpg",
		FileType:         "image",
		MimeType:         "image/jpeg",
		FileSize:         2048,
	}

	if err := db.CreateDocument(&outgoingDoc); err != nil {
		t.Fatalf("failed to create outgoing doc: %v", err)
	}

	// 3. Verify Stats
	totalDocs, totalIncoming, totalOutgoing, totalBytes, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if totalDocs != 2 {
		t.Errorf("expected totalDocs = 2, got %d", totalDocs)
	}
	if totalIncoming != 1 {
		t.Errorf("expected totalIncoming = 1, got %d", totalIncoming)
	}
	if totalOutgoing != 1 {
		t.Errorf("expected totalOutgoing = 1, got %d", totalOutgoing)
	}
	if totalBytes != 3072 {
		t.Errorf("expected totalBytes = 3072, got %d", totalBytes)
	}

	// 4. Test Filtering by DocType: incoming
	incomingOnly, err := db.GetDocuments(models.FilterParams{DocType: "incoming"})
	if err != nil {
		t.Fatalf("GetDocuments(incoming) failed: %v", err)
	}
	if len(incomingOnly) != 1 || incomingOnly[0].DocType != "incoming" {
		t.Errorf("expected 1 incoming doc, got %d", len(incomingOnly))
	}
	if incomingOnly[0].ReferenceNumber() != "101" {
		t.Errorf("expected reference number 101, got %s", incomingOnly[0].ReferenceNumber())
	}

	// 5. Test Filtering by DocType: outgoing
	outgoingOnly, err := db.GetDocuments(models.FilterParams{DocType: "outgoing"})
	if err != nil {
		t.Fatalf("GetDocuments(outgoing) failed: %v", err)
	}
	if len(outgoingOnly) != 1 || outgoingOnly[0].DocType != "outgoing" {
		t.Errorf("expected 1 outgoing doc, got %d", len(outgoingOnly))
	}
	if outgoingOnly[0].ReferenceNumber() != "742/ص" {
		t.Errorf("expected reference number 742/ص, got %s", outgoingOnly[0].ReferenceNumber())
	}

	// 6. Test Multi-word Search
	searchResults, err := db.GetDocuments(models.FilterParams{Query: "الماجستير 742"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(searchResults) != 1 || searchResults[0].ID != outgoingDoc.ID {
		t.Errorf("expected 1 search result matching outgoing doc, got %d", len(searchResults))
	}

	// 7. Test Download Filename
	if incomingDoc.DownloadFilename() != "101-طلب تثبيت موظفين.pdf" {
		t.Errorf("expected download name '101-طلب تثبيت موظفين.pdf', got '%s'", incomingDoc.DownloadFilename())
	}
	if outgoingDoc.DownloadFilename() != "742-ص-إيفاد رسمي لدراسة الماجستير.jpg" {
		t.Errorf("expected download name '742-ص-إيفاد رسمي لدراسة الماجستير.jpg', got '%s'", outgoingDoc.DownloadFilename())
	}

	// 8. Test Update Outgoing Doc
	outgoingDoc.Subject = "إيفاد رسمي معدل"
	if err := db.UpdateDocument(&outgoingDoc); err != nil {
		t.Fatalf("failed to update outgoing doc: %v", err)
	}
	updated, err := db.GetDocumentByID(outgoingDoc.ID)
	if err != nil {
		t.Fatalf("GetDocumentByID failed: %v", err)
	}
	if updated.Subject != "إيفاد رسمي معدل" {
		t.Errorf("expected updated subject, got %s", updated.Subject)
	}

	// 9. Test Delete
	filename, err := db.DeleteDocument(incomingDoc.ID)
	if err != nil {
		t.Fatalf("failed to delete doc: %v", err)
	}
	if filename != "test_in.pdf" {
		t.Errorf("expected filename 'test_in.pdf', got '%s'", filename)
	}

	remaining, _ := db.GetDocuments(models.FilterParams{DocType: "all"})
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining doc, got %d", len(remaining))
	}
}

func TestBackupTimeTracking(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer db.Close()

	targetTime := time.Now().Add(-6 * 24 * time.Hour) // 6 days ago
	if err := db.SetLastBackupTime(targetTime); err != nil {
		t.Fatalf("SetLastBackupTime failed: %v", err)
	}

	got, err := db.GetLastBackupTime()
	if err != nil {
		t.Fatalf("GetLastBackupTime failed after set: %v", err)
	}
	if got.Format("2006-01-02 15:04:05") != targetTime.Format("2006-01-02 15:04:05") {
		t.Errorf("expected backup time %v, got %v", targetTime, got)
	}
}

func TestWALTruncateOnClose(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "archive_wal_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_wal.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	doc := &models.Document{
		DocType:      "incoming",
		SerialNumber: "WAL-1",
		DocDate:      "2026-09-28",
		Department:   "قسم الحاسبة",
		Subject:      "فحص تفريغ WAL",
		Filename:     "wal_test.pdf",
	}
	if err := db.CreateDocument(doc); err != nil {
		t.Fatalf("failed to insert doc: %v", err)
	}

	// Close database handle - should execute PRAGMA wal_checkpoint(TRUNCATE)
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close failed: %v", err)
	}

	// In SQLite WAL mode with TRUNCATE, the WAL file if present should be 0 bytes
	walPath := dbPath + "-wal"
	if fi, err := os.Stat(walPath); err == nil {
		if fi.Size() > 0 {
			t.Errorf("expected WAL file to be truncated to 0 bytes on close, but size is %d", fi.Size())
		}
	}

	// Reopen and ensure data is safely persisted
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db2.Close()

	retrieved, err := db2.GetDocumentByID(doc.ID)
	if err != nil {
		t.Fatalf("failed to retrieve doc after close: %v", err)
	}
	if retrieved.SerialNumber != "WAL-1" {
		t.Errorf("expected serial 'WAL-1', got '%s'", retrieved.SerialNumber)
	}
}

func TestInitUserVersion(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer db.Close()

	var uv int
	if err := db.RawDB().QueryRow("PRAGMA user_version;").Scan(&uv); err != nil {
		t.Fatalf("failed to read user_version: %v", err)
	}
	if uv != 1 {
		t.Errorf("expected user_version to be 1 after Init, got %d", uv)
	}
}

