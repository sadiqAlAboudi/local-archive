package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"local-archive/internal/models"

	_ "modernc.org/sqlite"
)

// DB wraps the SQL database handle.
type DB struct {
	db *sql.DB
}

// Open connects to SQLite with WAL mode and busy timeout.
func Open(dbPath string) (*DB, error) {
	connStr := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &DB{db: db}
	if err := database.Init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return database, nil
}

// RawDB returns the underlying *sql.DB.
func (d *DB) RawDB() *sql.DB {
	return d.db
}

// Close closes the database connection.
func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Reopen closes the current database connection and opens a new connection to dbPath.
func (d *DB) Reopen(dbPath string) error {
	if d.db != nil {
		_ = d.db.Close()
	}
	connStr := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	newDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}
	d.db = newDB
	return d.Init()
}

// Init sets up tables and runs schema migrations.
func (d *DB) Init() error {
	queries := []string{
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
		`CREATE INDEX IF NOT EXISTS idx_documents_serial ON documents(serial_number);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_issue ON documents(issue_number);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_type ON documents(doc_type);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_dept ON documents(department);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_letterno ON documents(letter_number);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_subject ON documents(subject);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
		// Clean up obsolete tables from legacy web application
		`DROP TABLE IF EXISTS sessions;`,
		`DROP TABLE IF EXISTS users;`,
	}

	for _, q := range queries {
		if _, err := d.db.Exec(q); err != nil {
			return err
		}
	}

	// Schema migrations for backward compatibility
	_, _ = d.db.Exec("ALTER TABLE documents ADD COLUMN doc_type TEXT DEFAULT 'incoming';")
	_, _ = d.db.Exec("ALTER TABLE documents ADD COLUMN issue_number TEXT DEFAULT '';")
	_, _ = d.db.Exec("UPDATE documents SET doc_type = 'incoming' WHERE doc_type IS NULL OR doc_type = '';")

	return nil
}

// GetStats returns summary counts for all documents, incoming, outgoing, and total file size.
func (d *DB) GetStats() (totalDocs, totalIncoming, totalOutgoing, totalBytes int64, err error) {
	err = d.db.QueryRow(`
		SELECT 
			COUNT(*),
			COALESCE(SUM(CASE WHEN doc_type = 'incoming' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN doc_type = 'outgoing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(file_size), 0)
		FROM documents
	`).Scan(&totalDocs, &totalIncoming, &totalOutgoing, &totalBytes)
	return
}

// GetDocuments queries documents using filters.
func (d *DB) GetDocuments(params models.FilterParams) ([]models.Document, error) {
	var conditions []string
	var args []interface{}

	if params.DocType != "" && params.DocType != "all" {
		conditions = append(conditions, "doc_type = ?")
		args = append(args, params.DocType)
	}

	if params.Query != "" {
		terms := strings.Fields(params.Query)
		for _, term := range terms {
			like := "%" + term + "%"
			conditions = append(conditions, "(serial_number LIKE ? OR issue_number LIKE ? OR department LIKE ? OR letter_number LIKE ? OR subject LIKE ? OR original_filename LIKE ?)")
			args = append(args, like, like, like, like, like, like)
		}
	}

	query := `
		SELECT id, doc_type, serial_number, issue_number, doc_date, department, letter_number, letter_date, subject, filename, original_filename, file_type, mime_type, file_size, created_at, updated_at
		FROM documents
	`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY id DESC"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []models.Document
	for rows.Next() {
		var doc models.Document
		var createdAtStr, updatedAtStr string
		err := rows.Scan(
			&doc.ID,
			&doc.DocType,
			&doc.SerialNumber,
			&doc.IssueNumber,
			&doc.DocDate,
			&doc.Department,
			&doc.LetterNumber,
			&doc.LetterDate,
			&doc.Subject,
			&doc.Filename,
			&doc.OriginalFilename,
			&doc.FileType,
			&doc.MimeType,
			&doc.FileSize,
			&createdAtStr,
			&updatedAtStr,
		)
		if err != nil {
			return nil, err
		}

		doc.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		doc.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAtStr)
		docs = append(docs, doc)
	}

	return docs, rows.Err()
}

// GetDocumentByID retrieves a single document by ID.
func (d *DB) GetDocumentByID(id int64) (*models.Document, error) {
	var doc models.Document
	var createdAtStr, updatedAtStr string
	err := d.db.QueryRow(`
		SELECT id, doc_type, serial_number, issue_number, doc_date, department, letter_number, letter_date, subject, filename, original_filename, file_type, mime_type, file_size, created_at, updated_at
		FROM documents WHERE id = ?
	`, id).Scan(
		&doc.ID,
		&doc.DocType,
		&doc.SerialNumber,
		&doc.IssueNumber,
		&doc.DocDate,
		&doc.Department,
		&doc.LetterNumber,
		&doc.LetterDate,
		&doc.Subject,
		&doc.Filename,
		&doc.OriginalFilename,
		&doc.FileType,
		&doc.MimeType,
		&doc.FileSize,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	doc.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	doc.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAtStr)
	return &doc, nil
}

// CreateDocument inserts a new document record.
func (d *DB) CreateDocument(doc *models.Document) error {
	res, err := d.db.Exec(`
		INSERT INTO documents (doc_type, serial_number, issue_number, doc_date, department, letter_number, letter_date, subject, filename, original_filename, file_type, mime_type, file_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, doc.DocType, doc.SerialNumber, doc.IssueNumber, doc.DocDate, doc.Department, doc.LetterNumber, doc.LetterDate, doc.Subject, doc.Filename, doc.OriginalFilename, doc.FileType, doc.MimeType, doc.FileSize)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		doc.ID = id
	}
	return nil
}

// UpdateDocument updates an existing document record.
func (d *DB) UpdateDocument(doc *models.Document) error {
	_, err := d.db.Exec(`
		UPDATE documents 
		SET doc_type = ?, serial_number = ?, issue_number = ?, doc_date = ?, department = ?, letter_number = ?, letter_date = ?, subject = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, doc.DocType, doc.SerialNumber, doc.IssueNumber, doc.DocDate, doc.Department, doc.LetterNumber, doc.LetterDate, doc.Subject, doc.ID)
	return err
}

// DeleteDocument removes a document from the database and returns its stored filename.
func (d *DB) DeleteDocument(id int64) (string, error) {
	var filename string
	err := d.db.QueryRow("SELECT filename FROM documents WHERE id = ?", id).Scan(&filename)
	if err != nil {
		return "", err
	}

	_, err = d.db.Exec("DELETE FROM documents WHERE id = ?", id)
	if err != nil {
		return "", err
	}

	return filename, nil
}

// GetLastBackupTime retrieves the timestamp of the last recorded backup.
func (d *DB) GetLastBackupTime() (time.Time, error) {
	var val string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = 'last_backup_at'").Scan(&val)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, val)
}

// SetLastBackupTime records the timestamp of a successful backup.
func (d *DB) SetLastBackupTime(t time.Time) error {
	val := t.Format(time.RFC3339)
	_, err := d.db.Exec(`
		INSERT INTO settings (key, value) VALUES ('last_backup_at', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, val)
	return err
}
