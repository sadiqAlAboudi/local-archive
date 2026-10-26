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

func TestDateFiltering(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer db.Close()

	// Seed documents with various dates
	doc1 := &models.Document{
		DocType:          "incoming",
		SerialNumber:     "DOC-1",
		DocDate:          "2026-10-10",
		Department:       "Finance",
		LetterNumber:     "L-100",
		LetterDate:       "2026-10-08",
		Subject:          "Budget Proposal",
		Filename:         "doc1.pdf",
		OriginalFilename: "doc1.pdf",
		FileType:         "pdf",
		MimeType:         "application/pdf",
		FileSize:         500,
	}
	doc2 := &models.Document{
		DocType:          "outgoing",
		IssueNumber:      "OUT-2",
		DocDate:          "2026-10-15",
		Department:       "HR",
		Subject:          "Staff Notice",
		Filename:         "doc2.pdf",
		OriginalFilename: "doc2.pdf",
		FileType:         "pdf",
		MimeType:         "application/pdf",
		FileSize:         600,
	}
	doc3 := &models.Document{
		DocType:          "incoming",
		SerialNumber:     "DOC-3",
		DocDate:          "2026-10-20",
		Department:       "Legal",
		LetterNumber:     "L-300",
		LetterDate:       "2026-10-18",
		Subject:          "Contract Review",
		Filename:         "doc3.pdf",
		OriginalFilename: "doc3.pdf",
		FileType:         "pdf",
		MimeType:         "application/pdf",
		FileSize:         700,
	}

	for _, d := range []*models.Document{doc1, doc2, doc3} {
		if err := db.CreateDocument(d); err != nil {
			t.Fatalf("failed to create document: %v", err)
		}
	}

	// 1. Single Date Filter (exact match on 2026-10-15)
	singleDateDocs, err := db.GetDocuments(models.FilterParams{
		StartDate: "2026-10-15",
		EndDate:   "2026-10-15",
	})
	if err != nil {
		t.Fatalf("single date filter failed: %v", err)
	}
	if len(singleDateDocs) != 1 || singleDateDocs[0].ID != doc2.ID {
		t.Fatalf("expected 1 doc matching 2026-10-15, got %d", len(singleDateDocs))
	}

	// 2. Date Range Filter (2026-10-10 to 2026-10-16 -> doc1 and doc2)
	rangeDocs, err := db.GetDocuments(models.FilterParams{
		StartDate: "2026-10-10",
		EndDate:   "2026-10-16",
	})
	if err != nil {
		t.Fatalf("date range filter failed: %v", err)
	}
	if len(rangeDocs) != 2 {
		t.Fatalf("expected 2 docs in range, got %d", len(rangeDocs))
	}

	// 3. Inverted Date Range Filter (StartDate > EndDate should auto-swap)
	invertedDocs, err := db.GetDocuments(models.FilterParams{
		StartDate: "2026-10-22",
		EndDate:   "2026-10-12",
	})
	if err != nil {
		t.Fatalf("inverted date range filter failed: %v", err)
	}
	if len(invertedDocs) != 2 { // doc2 (10-15) and doc3 (10-20)
		t.Fatalf("expected 2 docs in swapped range, got %d", len(invertedDocs))
	}

	// 4. Open-ended StartDate only (>= 2026-10-15 -> doc2 and doc3)
	afterDocs, err := db.GetDocuments(models.FilterParams{
		StartDate: "2026-10-15",
	})
	if err != nil {
		t.Fatalf("start date only filter failed: %v", err)
	}
	if len(afterDocs) != 2 {
		t.Fatalf("expected 2 docs >= 2026-10-15, got %d", len(afterDocs))
	}

	// 5. Open-ended EndDate only (<= 2026-10-12 -> doc1)
	beforeDocs, err := db.GetDocuments(models.FilterParams{
		EndDate: "2026-10-12",
	})
	if err != nil {
		t.Fatalf("end date only filter failed: %v", err)
	}
	if len(beforeDocs) != 1 || beforeDocs[0].ID != doc1.ID {
		t.Fatalf("expected 1 doc <= 2026-10-12, got %d", len(beforeDocs))
	}

	// 6. Combination: DocType + Date Range
	comboDocs, err := db.GetDocuments(models.FilterParams{
		DocType:   "incoming",
		StartDate: "2026-10-01",
		EndDate:   "2026-10-16",
	})
	if err != nil {
		t.Fatalf("combo filter failed: %v", err)
	}
	if len(comboDocs) != 1 || comboDocs[0].ID != doc1.ID {
		t.Fatalf("expected 1 incoming doc in range, got %d", len(comboDocs))
	}

	// 7. LetterDate filter
	letterDocs, err := db.GetDocuments(models.FilterParams{
		StartDate: "2026-10-18",
		EndDate:   "2026-10-18",
		DateField: "letter_date",
	})
	if err != nil {
		t.Fatalf("letter date filter failed: %v", err)
	}
	if len(letterDocs) != 1 || letterDocs[0].ID != doc3.ID {
		t.Fatalf("expected 1 doc matching letter_date 2026-10-18, got %d", len(letterDocs))
	}
}

