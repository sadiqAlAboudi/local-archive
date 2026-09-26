package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"local-archive/internal/backup"
	"local-archive/internal/models"
	"local-archive/internal/sysutil"
)

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	docType := strings.TrimSpace(r.URL.Query().Get("type"))
	filterSerial := strings.TrimSpace(r.URL.Query().Get("serial"))
	filterDept := strings.TrimSpace(r.URL.Query().Get("dept"))
	filterLetter := strings.TrimSpace(r.URL.Query().Get("letterno"))
	filterDate := strings.TrimSpace(r.URL.Query().Get("date"))
	filterSubject := strings.TrimSpace(r.URL.Query().Get("subject"))

	if docType == "" {
		docType = "all"
	}

	filterParams := models.FilterParams{
		Query:         searchQuery,
		DocType:       docType,
		FilterSerial:  filterSerial,
		FilterDept:    filterDept,
		FilterLetter:  filterLetter,
		FilterDate:    filterDate,
		FilterSubject: filterSubject,
	}

	totalDocs, totalIncoming, totalOutgoing, totalBytes, err := a.db.GetStats()
	if err != nil {
		http.Error(w, "خطأ في قراءة إحصائيات الأرشيف: "+err.Error(), http.StatusInternalServerError)
		return
	}

	docs, err := a.db.GetDocuments(filterParams)
	if err != nil {
		http.Error(w, "خطأ في استعلام الوثائق: "+err.Error(), http.StatusInternalServerError)
		return
	}

	restoreSuccess := r.URL.Query().Get("restored") == "1"
	restoreError := strings.TrimSpace(r.URL.Query().Get("restore_error"))

	data := models.IndexViewData{
		TotalDocs:      totalDocs,
		TotalIncoming:  totalIncoming,
		TotalOutgoing:  totalOutgoing,
		StorageUsed:    sysutil.FormatBytes(totalBytes),
		Query:          searchQuery,
		FilterType:     docType,
		FilterSerial:   filterSerial,
		FilterDept:     filterDept,
		FilterLetter:   filterLetter,
		FilterDate:     filterDate,
		FilterSubject:  filterSubject,
		HasFilter:      filterParams.HasActiveFilters(),
		RestoreSuccess: restoreSuccess,
		RestoreError:   restoreError,
		Documents:      docs,
	}

	a.renderTemplate(w, "index.html", data)
}

func (a *App) handleView(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	doc, err := a.db.GetDocumentByID(id)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	a.renderTemplate(w, "view.html", models.ViewDocData{Doc: *doc})
}

func (a *App) handleServeFile(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	doc, err := a.db.GetDocumentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(a.uploadDir, doc.Filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", doc.MimeType)
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, filePath)
}

func (a *App) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	doc, err := a.db.GetDocumentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(a.uploadDir, doc.Filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	downloadName := doc.DownloadFilename()
	encodedName := url.PathEscape(downloadName)

	w.Header().Set("Content-Type", doc.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, downloadName, encodedName))
	http.ServeFile(w, r, filePath)
}

func (a *App) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 50 MB limit
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "تجاوز حجم الملف الحد المسموح أو تعذر قراءة النموذج", http.StatusBadRequest)
		return
	}

	docType := strings.TrimSpace(r.FormValue("doc_type"))
	if docType != "outgoing" {
		docType = "incoming"
	}

	serialNumber := strings.TrimSpace(r.FormValue("serial_number"))
	issueNumber := strings.TrimSpace(r.FormValue("issue_number"))
	docDate := strings.TrimSpace(r.FormValue("doc_date"))
	department := strings.TrimSpace(r.FormValue("department"))
	letterNumber := strings.TrimSpace(r.FormValue("letter_number"))
	letterDate := strings.TrimSpace(r.FormValue("letter_date"))
	subject := strings.TrimSpace(r.FormValue("subject"))

	// Validation based on type
	if docType == "outgoing" {
		if issueNumber == "" || subject == "" || docDate == "" || department == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		// Clear incoming-specific fields for outgoing
		serialNumber = ""
		letterNumber = ""
		letterDate = ""
	} else {
		if serialNumber == "" || subject == "" || docDate == "" || department == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		issueNumber = ""
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "الملف المرفق مطلوب", http.StatusBadRequest)
		return
	}
	defer file.Close()

	originalFilename := header.Filename
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
	case ".gif":
		fileType = "image"
		mimeType = "image/gif"
	default:
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		file.Seek(0, io.SeekStart)
		detected := http.DetectContentType(buf[:n])
		if strings.HasPrefix(detected, "image/") {
			fileType = "image"
			mimeType = detected
		} else if detected == "application/pdf" {
			fileType = "pdf"
			mimeType = detected
		} else {
			http.Error(w, "صيغة الملف غير مدعومة. يرجى رفع ملف PDF أو صورة.", http.StatusBadRequest)
			return
		}
	}

	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	storageName := fmt.Sprintf("%d_%s%s", time.Now().Unix(), hex.EncodeToString(randBytes), ext)
	dstPath := filepath.Join(a.uploadDir, storageName)

	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "فشل حفظ الملف المرفق", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	writtenBytes, err := io.Copy(dst, file)
	if err != nil {
		http.Error(w, "فشل كتابة محتوى الملف", http.StatusInternalServerError)
		return
	}

	newDoc := models.Document{
		DocType:          docType,
		SerialNumber:     serialNumber,
		IssueNumber:      issueNumber,
		DocDate:          docDate,
		Department:       department,
		LetterNumber:     letterNumber,
		LetterDate:       letterDate,
		Subject:          subject,
		Filename:         storageName,
		OriginalFilename: originalFilename,
		FileType:         fileType,
		MimeType:         mimeType,
		FileSize:         writtenBytes,
	}

	if err := a.db.CreateDocument(&newDoc); err != nil {
		os.Remove(dstPath)
		http.Error(w, "فشل تسجيل بيانات الوثيقة: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleEditDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	docType := strings.TrimSpace(r.FormValue("doc_type"))
	if docType != "outgoing" {
		docType = "incoming"
	}

	serialNumber := strings.TrimSpace(r.FormValue("serial_number"))
	issueNumber := strings.TrimSpace(r.FormValue("issue_number"))
	docDate := strings.TrimSpace(r.FormValue("doc_date"))
	department := strings.TrimSpace(r.FormValue("department"))
	letterNumber := strings.TrimSpace(r.FormValue("letter_number"))
	letterDate := strings.TrimSpace(r.FormValue("letter_date"))
	subject := strings.TrimSpace(r.FormValue("subject"))

	if docType == "outgoing" {
		if issueNumber == "" || subject == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		serialNumber = ""
		letterNumber = ""
		letterDate = ""
	} else {
		if serialNumber == "" || subject == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		issueNumber = ""
	}

	doc := models.Document{
		ID:           id,
		DocType:      docType,
		SerialNumber: serialNumber,
		IssueNumber:  issueNumber,
		DocDate:      docDate,
		Department:   department,
		LetterNumber: letterNumber,
		LetterDate:   letterDate,
		Subject:      subject,
	}

	_ = a.db.UpdateDocument(&doc)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	filename, err := a.db.DeleteDocument(id)
	if err == nil && filename != "" {
		filePath := filepath.Join(a.uploadDir, filename)
		os.Remove(filePath)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleBackup(w http.ResponseWriter, r *http.Request) {
	a.backupMutex.Lock()
	defer a.backupMutex.Unlock()

	timestamp := time.Now().Format("2006-01-02_150405")
	tempZipName := fmt.Sprintf("archive_backup_%s_%d.zip", timestamp, time.Now().UnixNano())
	tempZipPath := filepath.Join(os.TempDir(), tempZipName)
	defer os.Remove(tempZipPath)

	zipFile, err := os.Create(tempZipPath)
	if err != nil {
		http.Error(w, "تعذر إنشاء ملف النسخة الاحتياطية", http.StatusInternalServerError)
		return
	}

	err = backup.CreateFullBackup(a.db.RawDB(), a.dbPath, a.uploadDir, zipFile)
	zipFile.Close()

	if err != nil {
		http.Error(w, "فشل إنشاء النسخة الاحتياطية الشاملة: "+err.Error(), http.StatusInternalServerError)
		return
	}

	downloadFilename := fmt.Sprintf("archive_backup_%s.zip", timestamp)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadFilename))
	http.ServeFile(w, r, tempZipPath)
}

func (a *App) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	a.backupMutex.Lock()
	defer a.backupMutex.Unlock()

	// 500 MB limit for restoring archives
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Redirect(w, r, "/?restore_error="+url.QueryEscape("تجاوز حجم الملف الحد المسموح به"), http.StatusSeeOther)
		return
	}

	file, _, err := r.FormFile("backup_file")
	if err != nil {
		http.Redirect(w, r, "/?restore_error="+url.QueryEscape("يرجى اختيار ملف النسخة الاحتياطية"), http.StatusSeeOther)
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "uploaded_backup_*")
	if err != nil {
		http.Redirect(w, r, "/?restore_error="+url.QueryEscape("تعذر حفظ ملف النسخة الاحتياطية المؤقت"), http.StatusSeeOther)
		return
	}
	tempFilePath := tempFile.Name()
	defer os.Remove(tempFilePath)

	if _, err := io.Copy(tempFile, file); err != nil {
		tempFile.Close()
		http.Redirect(w, r, "/?restore_error="+url.QueryEscape("فشل استلام محتوى ملف النسخة الاحتياطية"), http.StatusSeeOther)
		return
	}
	tempFile.Close()

	if err := backup.RestoreArchiveBackup(a.dbPath, a.uploadDir, tempFilePath); err != nil {
		http.Redirect(w, r, "/?restore_error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	if err := a.db.Reopen(a.dbPath); err != nil {
		http.Redirect(w, r, "/?restore_error="+url.QueryEscape("تم استعادة الملفات ولكن تعذر إعادة تهيئة قاعدة البيانات: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?restored=1", http.StatusSeeOther)
}
