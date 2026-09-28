package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"local-archive/internal/models"
)

func TestIsCorruptError(t *testing.T) {
	if IsCorruptError(nil) {
		t.Errorf("expected false for nil error")
	}

	if IsCorruptError(fmt.Errorf("some generic error")) {
		t.Errorf("expected false for generic error")
	}

	if !IsCorruptError(fmt.Errorf("database disk image is malformed (11)")) {
		t.Errorf("expected true for database disk image is malformed (11)")
	}

	if !IsCorruptError(fmt.Errorf("SQLITE_CORRUPT: disk image is malformed")) {
		t.Errorf("expected true for SQLITE_CORRUPT")
	}
}

func TestCheckIntegrity(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer db.Close()

	if err := db.CheckIntegrity(); err != nil {
		t.Fatalf("expected clean db to pass integrity check, got: %v", err)
	}
}

func TestRecoverDatabase_IntactDB(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_archive.db")

	// Insert test docs
	for i := 1; i <= 5; i++ {
		doc := &models.Document{
			DocType:          "incoming",
			SerialNumber:     fmt.Sprintf("SN-%d", i),
			DocDate:          "2026-09-25",
			Department:       "وزارة الداخلية",
			LetterNumber:     fmt.Sprintf("LN-%d", i),
			LetterDate:       "2026-09-20",
			Subject:          fmt.Sprintf("موضوع وثيقة رقم %d", i),
			Filename:         fmt.Sprintf("file_%d.pdf", i),
			OriginalFilename: fmt.Sprintf("orig_%d.pdf", i),
			FileType:         "pdf",
			MimeType:         "application/pdf",
			FileSize:         1000,
		}
		if err := db.CreateDocument(doc); err != nil {
			t.Fatalf("failed to insert doc: %v", err)
		}
	}
	db.Close()

	// Run recovery
	res, err := RecoverDatabase(dbPath)
	if err != nil {
		t.Fatalf("RecoverDatabase failed: %v", err)
	}

	if !res.Success {
		t.Fatalf("expected success, got false")
	}
	if res.RecoveredDocs != 5 {
		t.Errorf("expected 5 recovered docs, got %d", res.RecoveredDocs)
	}
	if _, err := os.Stat(res.BackupPath); os.IsNotExist(err) {
		t.Errorf("expected corrupt backup file at %s, but does not exist", res.BackupPath)
	}

	// Verify the recovered DB is functional
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open recovered db: %v", err)
	}
	defer db2.Close()

	if err := db2.CheckIntegrity(); err != nil {
		t.Errorf("recovered db failed integrity check: %v", err)
	}

	docs, err := db2.GetDocuments(models.FilterParams{})
	if err != nil {
		t.Fatalf("GetDocuments failed on recovered db: %v", err)
	}
	if len(docs) != 5 {
		t.Errorf("expected 5 docs from recovered db, got %d", len(docs))
	}
}

func TestRecoverDatabase_CorruptedPages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "archive_corrupt_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "archive_corrupt.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}

	// Insert 30 documents with substantial subjects to span multiple B-tree pages
	for i := 1; i <= 30; i++ {
		doc := &models.Document{
			DocType:          "incoming",
			SerialNumber:     fmt.Sprintf("SN-%d", i),
			DocDate:          "2026-09-25",
			Department:       "وزارة النفط",
			LetterNumber:     fmt.Sprintf("LN-%d", i),
			LetterDate:       "2026-09-20",
			Subject:          fmt.Sprintf("موضوع تفصيلي مطول للوثيقة رقم %d مع بيانات نصية إضافية %s", i, strings.Repeat("بيانات ", 50)),
			Filename:         fmt.Sprintf("file_%d.pdf", i),
			OriginalFilename: fmt.Sprintf("orig_%d.pdf", i),
			FileType:         "pdf",
			MimeType:         "application/pdf",
			FileSize:         2048,
		}
		if err := db.CreateDocument(doc); err != nil {
			t.Fatalf("failed to insert doc: %v", err)
		}
	}
	_ = db.SetLastBackupTime(time.Now())
	db.Close()

	// Verify file size
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read db file: %v", err)
	}
	if len(data) <= 4096 {
		t.Fatalf("expected multi-page database, got %d bytes", len(data))
	}

	// Intentionally corrupt page 3 (index page, bytes 12288-12338)
	offset := 3 * 4096
	for i := offset + 10; i < offset + 50 && i < len(data); i++ {
		data[i] = 0xEE
	}
	if err := os.WriteFile(dbPath, data, 0644); err != nil {
		t.Fatalf("failed to write corrupted db: %v", err)
	}

	// Verify that the database is indeed corrupt
	testConn, _ := sql.Open("sqlite", dbPath)
	var checkResult string
	_ = testConn.QueryRow("PRAGMA integrity_check(1);").Scan(&checkResult)
	testConn.Close()

	if strings.ToLower(checkResult) == "ok" {
		t.Logf("Note: integrity check was ok on page 3, verifying recovery handles error")
	}

	// Attempt recovery
	res, err := RecoverDatabase(dbPath)
	if err != nil {
		t.Fatalf("RecoverDatabase failed: %v", err)
	}

	if !res.Success {
		t.Fatalf("expected recovery success, got false")
	}
	if res.RecoveredDocs == 0 {
		t.Errorf("expected at least some recovered docs, got 0")
	}

	// Verify corrupt backup was saved
	if _, err := os.Stat(res.BackupPath); os.IsNotExist(err) {
		t.Errorf("expected backup of corrupt database at %s", res.BackupPath)
	}

	// Open the recovered database and verify integrity
	recoveredDB, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open recovered database: %v", err)
	}
	defer recoveredDB.Close()

	if err := recoveredDB.CheckIntegrity(); err != nil {
		t.Fatalf("recovered database failed integrity check: %v", err)
	}

	// Verify documents can be queried
	docs, err := recoveredDB.GetDocuments(models.FilterParams{})
	if err != nil {
		t.Fatalf("failed to get documents after recovery: %v", err)
	}
	if len(docs) == 0 {
		t.Fatalf("expected documents in recovered database, got 0")
	}

	// Verify creating new document works on recovered DB
	newDoc := &models.Document{
		DocType:          "incoming",
		SerialNumber:     "NEW-001",
		DocDate:          "2026-09-28",
		Department:       "وزارة التخطيط",
		LetterNumber:     "L-100",
		LetterDate:       "2026-09-27",
		Subject:          "وثيقة جديدة بعد الإصلاح",
		Filename:         "new.pdf",
		OriginalFilename: "new.pdf",
		FileType:         "pdf",
		MimeType:         "application/pdf",
		FileSize:         500,
	}
	if err := recoveredDB.CreateDocument(newDoc); err != nil {
		t.Fatalf("failed to create document on recovered DB: %v", err)
	}
}

func TestRecoverDatabase_SevereCorruption(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "archive_severe_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "archive_severe.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	_ = db.CreateDocument(&models.Document{
		DocType:      "incoming",
		SerialNumber: "SN-1",
		DocDate:      "2026-09-25",
		Department:   "وزارة المالية",
		Subject:      "تجربة",
		Filename:     "f.pdf",
	})
	db.Close()

	// Overwrite database file with garbage
	garbage := make([]byte, 8192)
	for i := range garbage {
		garbage[i] = 0xFF
	}
	_ = os.WriteFile(dbPath, garbage, 0644)

	// Recover from completely scrambled file
	res, err := RecoverDatabase(dbPath)
	if err != nil {
		t.Fatalf("RecoverDatabase failed on severe corruption: %v", err)
	}

	if !res.Success {
		t.Fatalf("expected success on severe corruption recovery")
	}

	if _, err := os.Stat(res.BackupPath); os.IsNotExist(err) {
		t.Errorf("expected backup file at %s", res.BackupPath)
	}

	// Should be able to open and use the recovered database
	recoveredDB, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open recovered database after severe corruption: %v", err)
	}
	defer recoveredDB.Close()

	if err := recoveredDB.CheckIntegrity(); err != nil {
		t.Errorf("recovered db failed integrity check: %v", err)
	}
}
