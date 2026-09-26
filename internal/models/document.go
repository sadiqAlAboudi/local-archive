package models

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Document represents an archived incoming or outgoing document.
type Document struct {
	ID               int64     `json:"id"`
	DocType          string    `json:"doc_type"`          // "incoming" (كتاب وارد) or "outgoing" (كتاب صادر)
	SerialNumber     string    `json:"serial_number"`     // رقم التسلسل (لوارد)
	IssueNumber      string    `json:"issue_number"`      // العدد (لصادر)
	DocDate          string    `json:"doc_date"`          // التاريخ (تاريخ التسجيل للوارد، أو تاريخ الصدور للصادر)
	Department       string    `json:"department"`        // اسم الدائرة (الجهة الوارد منها أو الجهة الصادر إليها)
	LetterNumber     string    `json:"letter_number"`     // رقم الكتاب (لوارد)
	LetterDate       string    `json:"letter_date"`       // تاريخ الكتاب (لوارد)
	Subject          string    `json:"subject"`           // الموضوع
	Filename         string    `json:"filename"`          // Stored unique filename
	OriginalFilename string    `json:"original_filename"` // User original filename
	FileType         string    `json:"file_type"`         // "pdf" or "image"
	MimeType         string    `json:"mime_type"`
	FileSize         int64     `json:"file_size"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// FormattedSize returns a human-readable file size in Arabic.
func (d Document) FormattedSize() string {
	const unit = 1024
	b := d.FileSize
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

// FormattedCreatedAt returns formatted date string.
func (d Document) FormattedCreatedAt() string {
	return d.CreatedAt.Format("2006-01-02")
}

// DocTypeArabic returns Arabic description of the document type.
func (d Document) DocTypeArabic() string {
	if d.DocType == "outgoing" {
		return "كتاب صادر"
	}
	return "كتاب وارد"
}

// IsOutgoing returns true if the document is an outgoing letter.
func (d Document) IsOutgoing() bool {
	return d.DocType == "outgoing"
}

// ReferenceNumber returns the primary reference number (SerialNumber for incoming, IssueNumber for outgoing).
func (d Document) ReferenceNumber() string {
	if d.IsOutgoing() {
		return d.IssueNumber
	}
	return d.SerialNumber
}

// DownloadFilename returns the custom download filename based on doc type.
func (d Document) DownloadFilename() string {
	ref := d.ReferenceNumber()
	if strings.TrimSpace(ref) == "" {
		ref = "وثيقة"
	}
	ext := filepath.Ext(d.OriginalFilename)
	if ext == "" {
		ext = filepath.Ext(d.Filename)
	}
	name := fmt.Sprintf("%s-%s", ref, d.Subject)
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\r", "\n", "\t"}
	for _, inv := range invalid {
		name = strings.ReplaceAll(name, inv, "-")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "وثيقة"
	}
	return name + ext
}

// FilterParams holds filter and search query options.
type FilterParams struct {
	Query   string
	DocType string // "all", "incoming", "outgoing"
}

// HasActiveFilters returns true if search query or type filter is active.
func (f FilterParams) HasActiveFilters() bool {
	return f.Query != "" || (f.DocType != "" && f.DocType != "all")
}

// IndexViewData holds data for the main dashboard view.
type IndexViewData struct {
	TotalDocs          int64
	TotalIncoming      int64
	TotalOutgoing      int64
	StorageUsed        string
	Query              string
	FilterType         string // "all", "incoming", "outgoing"
	HasFilter          bool
	RestoreSuccess     bool
	RestoreError       string
	ShowBackupReminder bool
	LastBackupDays     int
	Documents          []Document
}

// ViewDocData holds data for single document preview.
type ViewDocData struct {
	Doc Document
}

// LoginViewData holds data for the login template.
type LoginViewData struct {
	Error             string
	ShowDefaultNotice bool
}

// ChangeCredsViewData holds data for changing credentials.
type ChangeCredsViewData struct {
	Error           string
	CurrentUsername string
}

// User represents a system administrator/user.
type User struct {
	ID                     int64
	Username               string
	PasswordHash           string
	MustChangeCredentials  int
	CreatedAt              time.Time
}

// Session represents an active login session.
type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}
