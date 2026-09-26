package backup

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCreateFullBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backup_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "archive.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	// Initialize simple schema and record
	if _, err := db.Exec("CREATE TABLE test_data (id INTEGER PRIMARY KEY, note TEXT);"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO test_data (note) VALUES ('hello backup');"); err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	// Create uploads directory with test files
	uploadDir := filepath.Join(tempDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		t.Fatalf("failed to create upload dir: %v", err)
	}

	file1Path := filepath.Join(uploadDir, "sample_document.pdf")
	file2Path := filepath.Join(uploadDir, "sample_image.png")
	if err := os.WriteFile(file1Path, []byte("%PDF-1.4 test document content"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte("PNG fake image bytes"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	// Run CreateFullBackup into a buffer
	var zipBuf bytes.Buffer
	if err := CreateFullBackup(db, dbPath, uploadDir, &zipBuf); err != nil {
		t.Fatalf("CreateFullBackup failed: %v", err)
	}

	// Validate zip contents
	zipReader, err := zip.NewReader(bytes.NewReader(zipBuf.Bytes()), int64(zipBuf.Len()))
	if err != nil {
		t.Fatalf("failed to read generated zip: %v", err)
	}

	foundEntries := make(map[string]bool)
	for _, f := range zipReader.File {
		foundEntries[f.Name] = true
	}

	if !foundEntries["archive.db"] {
		t.Errorf("expected zip to contain 'archive.db', got entries: %v", foundEntries)
	}
	if !foundEntries["uploads/sample_document.pdf"] {
		t.Errorf("expected zip to contain 'uploads/sample_document.pdf', got entries: %v", foundEntries)
	}
	if !foundEntries["uploads/sample_image.png"] {
		t.Errorf("expected zip to contain 'uploads/sample_image.png', got entries: %v", foundEntries)
	}

	// Verify that the restored archive.db from the zip can be opened and queried
	for _, f := range zipReader.File {
		if f.Name == "archive.db" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open archive.db from zip: %v", err)
			}
			restoredDBPath := filepath.Join(tempDir, "restored.db")
			outFile, err := os.Create(restoredDBPath)
			if err != nil {
				t.Fatalf("failed to create restored.db: %v", err)
			}
			io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()

			restoredDB, err := sql.Open("sqlite", restoredDBPath)
			if err != nil {
				t.Fatalf("failed to open restored.db: %v", err)
			}
			var note string
			err = restoredDB.QueryRow("SELECT note FROM test_data WHERE id = 1").Scan(&note)
			restoredDB.Close()
			if err != nil || note != "hello backup" {
				t.Errorf("restored db verification failed, note=%s, err=%v", note, err)
			}
			break
		}
	}

	// Test RestoreArchiveBackup into a separate target directory
	targetDir, err := os.MkdirTemp("", "restore_target_*")
	if err != nil {
		t.Fatalf("failed to create target temp dir: %v", err)
	}
	defer os.RemoveAll(targetDir)

	backupFilePath := filepath.Join(targetDir, "backup.zip")
	if err := os.WriteFile(backupFilePath, zipBuf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write backup zip file: %v", err)
	}

	targetDBPath := filepath.Join(targetDir, "archive.db")
	targetUploadDir := filepath.Join(targetDir, "uploads")

	if err := RestoreArchiveBackup(targetDBPath, targetUploadDir, backupFilePath); err != nil {
		t.Fatalf("RestoreArchiveBackup failed: %v", err)
	}

	// Verify restored db in target
	restoredDB, err := sql.Open("sqlite", targetDBPath)
	if err != nil {
		t.Fatalf("failed to open restored target db: %v", err)
	}
	var note string
	err = restoredDB.QueryRow("SELECT note FROM test_data WHERE id = 1").Scan(&note)
	restoredDB.Close()
	if err != nil || note != "hello backup" {
		t.Errorf("restored target db query failed: note=%s, err=%v", note, err)
	}

	// Verify restored uploads in target
	restoredPDF := filepath.Join(targetUploadDir, "sample_document.pdf")
	restoredPNG := filepath.Join(targetUploadDir, "sample_image.png")
	if _, err := os.Stat(restoredPDF); err != nil {
		t.Errorf("restored PDF not found: %v", err)
	}
	if _, err := os.Stat(restoredPNG); err != nil {
		t.Errorf("restored PNG not found: %v", err)
	}
}
