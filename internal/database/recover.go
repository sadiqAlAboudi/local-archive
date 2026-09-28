package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// RecoveryResult contains details of the database recovery operation.
type RecoveryResult struct {
	Success          bool   `json:"success"`
	Message          string `json:"message"`
	RecoveredDocs    int    `json:"recovered_docs"`
	CorruptedSkipped int    `json:"corrupted_skipped"`
	BackupPath       string `json:"backup_path"`
	StrategyUsed     string `json:"strategy_used"`
}

// IsCorruptError tests if an error indicates SQLite database corruption.
func IsCorruptError(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		if (sqliteErr.Code() & 0xFF) == sqlite3.SQLITE_CORRUPT { // Code 11
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database disk image is malformed") ||
		strings.Contains(msg, "disk image is malformed") ||
		strings.Contains(msg, "malformed (11)") ||
		strings.Contains(msg, "file is not a database")
}

// CheckIntegrity runs SQLite integrity check on the database.
func (d *DB) CheckIntegrity() error {
	if d.db == nil {
		return fmt.Errorf("قاعدة البيانات غير متصلة")
	}
	var res string
	err := d.db.QueryRow("PRAGMA integrity_check(1);").Scan(&res)
	if err != nil {
		return err
	}
	if strings.ToLower(res) != "ok" {
		return fmt.Errorf("فشل فحص سلامة قاعدة البيانات: %s", res)
	}
	return nil
}

// RecoverDatabase performs a multi-strategy recovery on a corrupted SQLite database.
// 1. Backs up the corrupted database and WAL/SHM files with a timestamp.
// 2. Attempts fast recovery via REINDEX and VACUUM INTO.
// 3. If that fails, extracts all readable rows and settings into a clean schema.
// 4. Replaces the corrupt file with the verified recovered database.
func RecoverDatabase(dbPath string) (*RecoveryResult, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ملف قاعدة البيانات غير موجود في المسار: %s", dbPath)
	}

	// 1. Backup the corrupted files
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.corrupt.%s.bak", dbPath, timestamp)
	if err := copyFile(dbPath, backupPath); err != nil {
		return nil, fmt.Errorf("تعذر إنشاء نسخة احتياطية من الملف التالف: %w", err)
	}
	if _, err := os.Stat(dbPath + "-wal"); err == nil {
		_ = copyFile(dbPath+"-wal", backupPath+"-wal")
	}
	if _, err := os.Stat(dbPath + "-shm"); err == nil {
		_ = copyFile(dbPath+"-shm", backupPath+"-shm")
	}

	result := &RecoveryResult{
		BackupPath: backupPath,
	}

	// 2. Strategy A: Fast Repair via REINDEX and VACUUM INTO
	vacErr := attemptVacuumRecovery(backupPath, dbPath, result)
	if vacErr == nil {
		result.Success = true
		result.StrategyUsed = "reindex_vacuum"
		result.Message = fmt.Sprintf("تم إصلاح قاعدة البيانات بنجاح واسترجاع %d وثيقة (تم حفظ نسخة تالفة في: %s)",
			result.RecoveredDocs, filepath.Base(backupPath))
		return result, nil
	}

	// 3. Strategy B: Deep Table & Row-by-Row Salvage
	salvageErr := attemptRowSalvage(backupPath, dbPath, result)
	if salvageErr == nil {
		result.Success = true
		result.StrategyUsed = "salvage_rows"
		result.Message = fmt.Sprintf("تم إنقاذ واسترجاع %d وثيقة وتخطي %d سجلات تالفة (تم حفظ نسخة تالفة في: %s)",
			result.RecoveredDocs, result.CorruptedSkipped, filepath.Base(backupPath))
		return result, nil
	}

	// 4. Strategy C: Clean Schema Reset (as last resort, keeping the corrupt backup safe)
	freshErr := initializeFreshDB(dbPath)
	if freshErr == nil {
		result.Success = true
		result.StrategyUsed = "fresh_schema"
		result.Message = fmt.Sprintf("تعذر قراءة صفحات الملف التالف نهائياً. تم إنشاء قاعدة بيانات جديدة ونظيفة مع الحفاظ على النسخة الأصلية في: %s",
			filepath.Base(backupPath))
		return result, nil
	}

	return nil, fmt.Errorf("فشلت جميع محاولات استرجاع قاعدة البيانات: %v", salvageErr)
}

// attemptVacuumRecovery attempts recovery by rebuilding indexes and vacuuming into a fresh file.
func attemptVacuumRecovery(sourcePath, destPath string, result *RecoveryResult) error {
	tempVacPath := filepath.Join(os.TempDir(), fmt.Sprintf("rec_vac_%d.db", time.Now().UnixNano()))
	defer os.Remove(tempVacPath)

	connStr := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=writable_schema(1)", sourcePath)
	srcDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return err
	}
	srcDB.SetMaxOpenConns(1)
	defer srcDB.Close()

	// Try reindexing
	_, _ = srcDB.Exec("REINDEX;")

	// Attempt VACUUM INTO
	slashPath := filepath.ToSlash(tempVacPath)
	_, err = srcDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", slashPath))
	if err != nil {
		return err
	}

	// Verify the vacuumed database integrity
	checkDB, err := sql.Open("sqlite", tempVacPath)
	if err != nil {
		return err
	}
	checkDB.SetMaxOpenConns(1)
	defer checkDB.Close()

	var checkRes string
	if err := checkDB.QueryRow("PRAGMA quick_check(1);").Scan(&checkRes); err != nil || strings.ToLower(checkRes) != "ok" {
		return fmt.Errorf("فشل فحص سلامة الملف بعد التفريغ: %v (%s)", err, checkRes)
	}

	var count int
	_ = checkDB.QueryRow("SELECT COUNT(*) FROM documents;").Scan(&count)
	result.RecoveredDocs = count

	checkDB.Close()
	srcDB.Close()

	// Replace destination with recovered database
	_ = os.Remove(destPath + "-wal")
	_ = os.Remove(destPath + "-shm")
	return copyFile(tempVacPath, destPath)
}

// attemptRowSalvage extracts every readable record from corrupt source into a new clean database.
func attemptRowSalvage(sourcePath, destPath string, result *RecoveryResult) error {
	tempCleanPath := filepath.Join(os.TempDir(), fmt.Sprintf("rec_salvage_%d.db", time.Now().UnixNano()))
	defer os.Remove(tempCleanPath)

	// Create new database with standard schema
	cleanDB, err := sql.Open("sqlite", tempCleanPath)
	if err != nil {
		return err
	}
	cleanDB.SetMaxOpenConns(1)
	defer cleanDB.Close()

	initSchema := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			doc_type TEXT NOT NULL DEFAULT 'incoming',
			serial_number TEXT NOT NULL DEFAULT '',
			issue_number TEXT NOT NULL DEFAULT '',
			doc_date TEXT NOT NULL,
			department TEXT NOT NULL,
			letter_number TEXT NOT NULL DEFAULT '',
			letter_date TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL,
			filename TEXT NOT NULL,
			original_filename TEXT NOT NULL,
			file_type TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
		`INSERT OR IGNORE INTO settings (key, value) VALUES ('installed_at', CURRENT_TIMESTAMP);`,
	}
	for _, q := range initSchema {
		if _, err := cleanDB.Exec(q); err != nil {
			return fmt.Errorf("failed to setup clean schema: %w", err)
		}
	}

	// Open corrupt source in read-only resilient mode
	corruptConn := fmt.Sprintf("%s?mode=ro&_pragma=writable_schema(1)&_pragma=ignore_check_constraints(1)&_pragma=query_only(1)", sourcePath)
	srcDB, err := sql.Open("sqlite", corruptConn)
	if err != nil {
		return err
	}
	srcDB.SetMaxOpenConns(1)
	defer srcDB.Close()

	// Gather known IDs
	idSet := make(map[int64]bool)
	var maxID int64

	rows, err := srcDB.Query("SELECT id FROM documents;")
	if err == nil {
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err == nil {
				idSet[id] = true
				if id > maxID {
					maxID = id
				}
			}
		}
		if rErr := rows.Err(); rErr != nil {
			// Expected when traversing corrupted database pages
			_ = rErr
		}
		rows.Close()
	}

	// Also check sqlite_sequence for documents
	var seqVal int64
	if err := srcDB.QueryRow("SELECT seq FROM sqlite_sequence WHERE name = 'documents';").Scan(&seqVal); err == nil {
		if seqVal > maxID {
			maxID = seqVal
		}
	}

	// If no maxID found, default to probe range
	if maxID == 0 {
		maxID = 5000
	} else {
		maxID += 100
	}

	insertDocStmt, err := cleanDB.Prepare(`
		INSERT OR REPLACE INTO documents (id, doc_type, serial_number, issue_number, doc_date, department, letter_number, letter_date, subject, filename, original_filename, file_type, mime_type, file_size, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer insertDocStmt.Close()

	recoveredCount := 0
	corruptCount := 0

	// Retrieve each record individually to bypass corrupted B-tree pages
	for id := int64(1); id <= maxID; id++ {
		row := srcDB.QueryRow(`
			SELECT id, doc_type, serial_number, issue_number, doc_date, department, letter_number, letter_date, subject, filename, original_filename, file_type, mime_type, file_size, created_at, updated_at
			FROM documents WHERE id = ?
		`, id)

		var (
			dID                                                             int64
			docType, serialNo, issueNo, docDate, dept, letterNo, letterDate string
			subj, filename, origFilename, fileType, mimeType, crAt, upAt    string
			fileSize                                                        int64
		)

		err := row.Scan(&dID, &docType, &serialNo, &issueNo, &docDate, &dept, &letterNo, &letterDate, &subj, &filename, &origFilename, &fileType, &mimeType, &fileSize, &crAt, &upAt)
		if err == nil {
			_, insErr := insertDocStmt.Exec(dID, docType, serialNo, issueNo, docDate, dept, letterNo, letterDate, subj, filename, origFilename, fileType, mimeType, fileSize, crAt, upAt)
			if insErr == nil {
				recoveredCount++
			}
		} else if IsCorruptError(err) {
			corruptCount++
		}
	}

	// Salvage settings
	sRows, sErr := srcDB.Query("SELECT key, value FROM settings;")
	if sErr == nil {
		for sRows.Next() {
			var k, v string
			if sRows.Scan(&k, &v) == nil {
				_, _ = cleanDB.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", k, v)
			}
		}
		if srErr := sRows.Err(); srErr != nil {
			// Expected when settings page is damaged
			_ = srErr
		}
		sRows.Close()
	}

	// Create indices on clean database
	indices := []string{
		`CREATE INDEX IF NOT EXISTS idx_documents_serial ON documents(serial_number);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_issue ON documents(issue_number);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_type ON documents(doc_type);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_dept ON documents(department);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_letterno ON documents(letter_number);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_subject ON documents(subject);`,
	}
	for _, idx := range indices {
		_, _ = cleanDB.Exec(idx)
	}

	cleanDB.Close()
	srcDB.Close()

	result.RecoveredDocs = recoveredCount
	result.CorruptedSkipped = corruptCount

	// Replace destination with salvaged database
	_ = os.Remove(destPath + "-wal")
	_ = os.Remove(destPath + "-shm")
	return copyFile(tempCleanPath, destPath)
}

// initializeFreshDB creates a new empty database with default schema.
func initializeFreshDB(dbPath string) error {
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	db, err := Open(dbPath)
	if err != nil {
		return err
	}
	return db.Close()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
