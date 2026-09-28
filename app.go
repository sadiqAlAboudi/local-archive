package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"local-archive/internal/backup"
	"local-archive/internal/database"
	"local-archive/internal/models"
	"local-archive/internal/sysutil"
	"local-archive/internal/updater"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App manages the desktop application lifecycle and exposes methods to frontend.
type App struct {
	ctx         context.Context
	db          *database.DB
	paths       sysutil.AppDataPaths
	backupMutex sync.Mutex
}

// NewApp creates a new App struct instance.
func NewApp(db *database.DB, paths sysutil.AppDataPaths) *App {
	return &App{
		db:    db,
		paths: paths,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		_ = a.db.Close()
	}
}

// SelectDocumentFile opens the OS native file dialog to choose a document/image.
func (a *App) SelectDocumentFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "اختر وثيقة للإرفاق",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "المستندات والصور (*.pdf, *.png, *.jpg, *.jpeg, *.webp)",
				Pattern:     "*.pdf;*.png;*.jpg;*.jpeg;*.webp",
			},
			{
				DisplayName: "جميع الملفات (*.*)",
				Pattern:     "*.*",
			},
		},
	})
}

// CreateDocumentInput defines the parameters passed from JavaScript.
type CreateDocumentInput struct {
	DocType      string `json:"doc_type"`
	SerialNumber string `json:"serial_number"`
	IssueNumber  string `json:"issue_number"`
	DocDate      string `json:"doc_date"`
	Department   string `json:"department"`
	LetterNumber string `json:"letter_number"`
	LetterDate   string `json:"letter_date"`
	Subject      string `json:"subject"`
	SourcePath   string `json:"source_path"`
}

// CreateDocument copies the selected local file and records the document in SQLite.
func (a *App) CreateDocument(input CreateDocumentInput) (*models.Document, error) {
	cleanSource := strings.TrimSpace(input.SourcePath)
	if cleanSource == "" {
		return nil, fmt.Errorf("يرجى اختيار ملف الوثيقة المرفق")
	}

	srcFile, err := os.Open(cleanSource)
	if err != nil {
		return nil, fmt.Errorf("تعذر فتح الملف المختار: %w", err)
	}
	defer srcFile.Close()

	srcStat, err := srcFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("تعذر قراءة بيانات الملف: %w", err)
	}

	originalFilename := filepath.Base(cleanSource)
	ext := strings.ToLower(filepath.Ext(originalFilename))

	var fileType string
	var mimeType string

	switch ext {
	case ".pdf":
		fileType = "pdf"
		mimeType = "application/pdf"
	case ".jpg", ".jpeg":
		fileType = "image"
		mimeType = "image/jpeg"
	case ".png":
		fileType = "image"
		mimeType = "image/png"
	case ".webp":
		fileType = "image"
		mimeType = "image/webp"
	default:
		buf := make([]byte, 512)
		n, _ := srcFile.Read(buf)
		srcFile.Seek(0, io.SeekStart)
		detected := http.DetectContentType(buf[:n])
		if strings.HasPrefix(detected, "image/") {
			fileType = "image"
			mimeType = detected
		} else if detected == "application/pdf" {
			fileType = "pdf"
			mimeType = detected
		} else {
			return nil, fmt.Errorf("صيغة الملف غير مدعومة. يرجى اختيار ملف PDF أو صورة")
		}
	}

	// Generate unique storage name
	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	storageName := fmt.Sprintf("%d_%s%s", time.Now().Unix(), hex.EncodeToString(randBytes), ext)
	dstPath := filepath.Join(a.paths.UploadDir, storageName)

	dst, err := os.Create(dstPath)
	if err != nil {
		return nil, fmt.Errorf("فشل حفظ الملف في مجلد التخزين: %w", err)
	}
	defer dst.Close()

	writtenBytes, err := io.Copy(dst, srcFile)
	if err != nil {
		os.Remove(dstPath)
		return nil, fmt.Errorf("فشل نسخ محتوى الملف: %w", err)
	}

	docType := strings.TrimSpace(input.DocType)
	if docType != "outgoing" {
		docType = "incoming"
	}

	serialNumber := strings.TrimSpace(input.SerialNumber)
	issueNumber := strings.TrimSpace(input.IssueNumber)
	letterNumber := strings.TrimSpace(input.LetterNumber)
	letterDate := strings.TrimSpace(input.LetterDate)

	if docType == "outgoing" {
		serialNumber = ""
		letterNumber = ""
		letterDate = ""
	} else {
		issueNumber = ""
	}

	newDoc := models.Document{
		DocType:          docType,
		SerialNumber:     serialNumber,
		IssueNumber:      issueNumber,
		DocDate:          strings.TrimSpace(input.DocDate),
		Department:       strings.TrimSpace(input.Department),
		LetterNumber:     letterNumber,
		LetterDate:       letterDate,
		Subject:          strings.TrimSpace(input.Subject),
		Filename:         storageName,
		OriginalFilename: originalFilename,
		FileType:         fileType,
		MimeType:         mimeType,
		FileSize:         writtenBytes,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if newDoc.DocDate == "" || newDoc.Department == "" || newDoc.Subject == "" {
		os.Remove(dstPath)
		return nil, fmt.Errorf("الحقول الأساسية (التاريخ، الدائرة، الموضوع) مطلوبة")
	}

	if docType == "outgoing" && newDoc.IssueNumber == "" {
		os.Remove(dstPath)
		return nil, fmt.Errorf("حقل العدد مطلوب للكتب الصادرة")
	}

	if docType == "incoming" && newDoc.SerialNumber == "" {
		os.Remove(dstPath)
		return nil, fmt.Errorf("حقل رقم التسلسل مطلوب للكتب الواردة")
	}

	if docType == "incoming" && newDoc.DocDate != "" && newDoc.LetterDate != "" {
		if isDocDateBeforeLetterDate(newDoc.DocDate, newDoc.LetterDate) {
			os.Remove(dstPath)
			return nil, fmt.Errorf("تاريخ ورود الكتاب (%s) لا يمكن أن يكون قبل تاريخ الكتاب الوارد (%s)", newDoc.DocDate, newDoc.LetterDate)
		}
	}

	if err := a.db.CreateDocument(&newDoc); err != nil {
		os.Remove(dstPath)
		return nil, fmt.Errorf("فشل تسجيل الوثيقة في قاعدة البيانات: %w", err)
	}

	_ = srcStat
	return &newDoc, nil
}

// isDocDateBeforeLetterDate checks if docDate is chronologically before letterDate.
func isDocDateBeforeLetterDate(docDate, letterDate string) bool {
	dDate, err1 := time.Parse("2006-01-02", strings.TrimSpace(docDate))
	lDate, err2 := time.Parse("2006-01-02", strings.TrimSpace(letterDate))
	if err1 == nil && err2 == nil {
		return dDate.Before(lDate)
	}
	return strings.TrimSpace(docDate) < strings.TrimSpace(letterDate)
}

// UpdateDocumentInput defines fields for updating an existing document.
type UpdateDocumentInput struct {
	ID           int64  `json:"id"`
	DocType      string `json:"doc_type"`
	SerialNumber string `json:"serial_number"`
	IssueNumber  string `json:"issue_number"`
	DocDate      string `json:"doc_date"`
	Department   string `json:"department"`
	LetterNumber string `json:"letter_number"`
	LetterDate   string `json:"letter_date"`
	Subject      string `json:"subject"`
}

// UpdateDocument updates an existing document record.
func (a *App) UpdateDocument(input UpdateDocumentInput) error {
	docType := strings.TrimSpace(input.DocType)
	if docType != "outgoing" {
		docType = "incoming"
	}

	serialNumber := strings.TrimSpace(input.SerialNumber)
	issueNumber := strings.TrimSpace(input.IssueNumber)
	letterNumber := strings.TrimSpace(input.LetterNumber)
	letterDate := strings.TrimSpace(input.LetterDate)

	if docType == "outgoing" {
		serialNumber = ""
		letterNumber = ""
		letterDate = ""
	} else {
		issueNumber = ""
	}

	doc := models.Document{
		ID:           input.ID,
		DocType:      docType,
		SerialNumber: serialNumber,
		IssueNumber:  issueNumber,
		DocDate:      strings.TrimSpace(input.DocDate),
		Department:   strings.TrimSpace(input.Department),
		LetterNumber: letterNumber,
		LetterDate:   letterDate,
		Subject:      strings.TrimSpace(input.Subject),
	}

	if docType == "incoming" && doc.DocDate != "" && doc.LetterDate != "" {
		if isDocDateBeforeLetterDate(doc.DocDate, doc.LetterDate) {
			return fmt.Errorf("تاريخ ورود الكتاب (%s) لا يمكن أن يكون قبل تاريخ الكتاب الوارد (%s)", doc.DocDate, doc.LetterDate)
		}
	}

	return a.db.UpdateDocument(&doc)
}

// DeleteDocument removes the document record and deletes its storage file.
func (a *App) DeleteDocument(id int64) error {
	filename, err := a.db.DeleteDocument(id)
	if err != nil {
		return err
	}

	if filename != "" {
		filePath := filepath.Join(a.paths.UploadDir, filename)
		_ = os.Remove(filePath)
	}

	return nil
}

// GetDocuments queries the documents list with filters.
func (a *App) GetDocuments(query, docType string) ([]models.Document, error) {
	return a.db.GetDocuments(models.FilterParams{
		Query:   strings.TrimSpace(query),
		DocType: strings.TrimSpace(docType),
	})
}

// GetDocument returns a single document by ID.
func (a *App) GetDocument(id int64) (*models.Document, error) {
	return a.db.GetDocumentByID(id)
}

// GetStats returns current system statistics.
func (a *App) GetStats() (models.IndexViewData, error) {
	totalDocs, totalIncoming, totalOutgoing, totalBytes, err := a.db.GetStats()
	if err != nil {
		return models.IndexViewData{}, err
	}

	var showReminder bool
	var lastBackupDays int
	if totalDocs > 0 {
		hasBackup, _ := a.db.HasBackupRecord()
		lastBackup, err := a.db.GetLastBackupTime()
		if err == nil && !lastBackup.IsZero() {
			days := int(time.Since(lastBackup).Hours() / 24)
			if days >= 5 {
				showReminder = true
				if hasBackup {
					lastBackupDays = days
				} else {
					lastBackupDays = 0
				}
			}
		}
	}

	return models.IndexViewData{
		TotalDocs:          totalDocs,
		TotalIncoming:      totalIncoming,
		TotalOutgoing:      totalOutgoing,
		StorageUsed:        sysutil.FormatBytes(totalBytes),
		ShowBackupReminder: showReminder,
		LastBackupDays:     lastBackupDays,
	}, nil
}

// CreateBackup opens a save dialog and creates a complete zip backup.
func (a *App) CreateBackup() (string, error) {
	a.backupMutex.Lock()
	defer a.backupMutex.Unlock()

	defaultName := fmt.Sprintf("archive_backup_%s.zip", time.Now().Format("2006-01-02_150405"))
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "حفظ النسخة الاحتياطية للأرشيف",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "ملف مضغوط (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil || savePath == "" {
		return "", err
	}

	f, err := os.Create(savePath)
	if err != nil {
		return "", fmt.Errorf("تعذر إنشاء ملف النسخة الاحتياطية: %w", err)
	}
	defer f.Close()

	if err := backup.CreateFullBackup(a.db.RawDB(), a.paths.DBPath, a.paths.UploadDir, f); err != nil {
		os.Remove(savePath)
		return "", fmt.Errorf("فشل إنشاء النسخة الاحتياطية الشاملة: %w", err)
	}

	_ = a.db.SetLastBackupTime(time.Now())
	return savePath, nil
}

// RestoreBackup opens a file dialog to select a backup zip and restores it.
func (a *App) RestoreBackup() error {
	a.backupMutex.Lock()
	defer a.backupMutex.Unlock()

	zipPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "اختر ملف النسخة الاحتياطية للاستعادة",
		Filters: []runtime.FileFilter{
			{DisplayName: "ملف النسخة الاحتياطية (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil || zipPath == "" {
		return err
	}

	if err := backup.RestoreArchiveBackup(a.paths.DBPath, a.paths.UploadDir, zipPath); err != nil {
		return fmt.Errorf("فشل استعادة النسخة الاحتياطية: %w", err)
	}

	if err := a.db.Reopen(a.paths.DBPath); err != nil {
		return fmt.Errorf("تمت استعادة الملفات بنجاح ولكن تعذر إعادة تهيئة قاعدة البيانات: %w", err)
	}

	_ = a.db.SetLastBackupTime(time.Now())
	return nil
}

// CheckDatabaseIntegrity verifies the integrity of the database using SQLite PRAGMA integrity_check.
func (a *App) CheckDatabaseIntegrity() (string, error) {
	if a.db == nil {
		return "قاعدة البيانات غير متصلة", fmt.Errorf("database handle is nil")
	}
	if err := a.db.CheckIntegrity(); err != nil {
		return err.Error(), err
	}
	return "قاعدة البيانات سليمة ولا توجد أخطاء", nil
}

// RecoverDatabase runs the recovery process on the database, preserving a backup of corrupt data.
func (a *App) RecoverDatabase() (*database.RecoveryResult, error) {
	a.backupMutex.Lock()
	defer a.backupMutex.Unlock()

	if a.db != nil {
		_ = a.db.Close()
	}

	result, err := database.RecoverDatabase(a.paths.DBPath)
	if err != nil {
		_ = a.db.Reopen(a.paths.DBPath)
		return nil, fmt.Errorf("فشلت عملية استرجاع قاعدة البيانات: %w", err)
	}

	if err := a.db.Reopen(a.paths.DBPath); err != nil {
		return nil, fmt.Errorf("تم استرجاع البيانات بنجاح ولكن تعذر إعادة الاتصال بقاعدة البيانات: %w", err)
	}

	return result, nil
}

// GetAppVersion returns the current application version.
func (a *App) GetAppVersion() string {
	return updater.CurrentVersion
}

// CheckForUpdate queries GitHub releases to see if a newer version is available.
func (a *App) CheckForUpdate() (*updater.UpdateInfo, error) {
	return updater.CheckForUpdates(nil, updater.CurrentVersion, updater.DefaultRepoOwner, updater.DefaultRepoName)
}

// DownloadAndApplyUpdate downloads LocalArchive-Setup.exe to %TEMP%,
// runs the installer silently, and exits the running app so files can be overwritten.
func (a *App) DownloadAndApplyUpdate() error {
	info, err := a.CheckForUpdate()
	if err != nil {
		return fmt.Errorf("فشل التحقق من وجود تحديث: %w", err)
	}

	if !info.Available {
		return fmt.Errorf("أنت تستخدم بالفعل أحدث إصدار متاح (%s)", updater.CurrentVersion)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "update:status", "downloading")
	}

	installerPath, err := updater.DownloadInstaller(ctx, nil, info.DownloadURL, func(downloaded, total int64, percent float64) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "update:progress", map[string]interface{}{
				"downloaded":          downloaded,
				"total":               total,
				"percent":             percent,
				"downloadedFormatted": sysutil.FormatBytes(downloaded),
				"totalFormatted":      sysutil.FormatBytes(total),
			})
		}
	})
	if err != nil {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "update:status", "error")
		}
		return fmt.Errorf("فشل تنزيل ملف التحديث: %w", err)
	}

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "update:status", "installing")
	}

	// Flush and close the database before shutting down
	if a.db != nil {
		_ = a.db.Close()
	}

	time.Sleep(300 * time.Millisecond)

	if err := updater.RunInstallerAndExit(installerPath); err != nil {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "update:status", "error")
		}
		return fmt.Errorf("فشل تشغيل مثبت التحديث: %w", err)
	}

	return nil
}

