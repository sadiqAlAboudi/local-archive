package backup

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// CreateFullBackup creates a comprehensive zip archive containing the SQLite database and all uploaded files.
func CreateFullBackup(db *sql.DB, dbPath, uploadDir string, w io.Writer) error {
	// 1. Create a safe SQLite snapshot to a temporary file using VACUUM INTO
	tempDir := os.TempDir()
	tempDBName := fmt.Sprintf("temp_db_backup_%d.db", time.Now().UnixNano())
	tempDBPath := filepath.Join(tempDir, tempDBName)
	defer os.Remove(tempDBPath)

	// Attempt VACUUM INTO
	_, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", filepath.ToSlash(tempDBPath)))
	if err != nil {
		// Fallback to file copy if VACUUM INTO is not supported or fails
		if copyErr := copyFile(dbPath, tempDBPath); copyErr != nil {
			return fmt.Errorf("failed to snapshot database: %w (fallback error: %v)", err, copyErr)
		}
	}

	// 2. Initialize Zip Writer
	zw := zip.NewWriter(w)
	defer zw.Close()

	// 3. Add archive.db to the root of the zip archive
	if err := addFileToZip(zw, tempDBPath, "archive.db"); err != nil {
		return fmt.Errorf("failed to add database to backup zip: %w", err)
	}

	// 4. Add all files in uploadDir to uploads/ within the zip archive
	if info, err := os.Stat(uploadDir); err == nil && info.IsDir() {
		err = filepath.Walk(uploadDir, func(path string, fileInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if fileInfo.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(uploadDir, path)
			if err != nil {
				return err
			}

			zipEntryPath := filepath.ToSlash(filepath.Join("uploads", relPath))
			return addFileToZip(zw, path, zipEntryPath)
		})

		if err != nil {
			return fmt.Errorf("failed to add uploads to backup zip: %w", err)
		}
	}

	return nil
}

// RestoreArchiveBackup restores the database and uploaded files from a .zip or .db backup file.
func RestoreArchiveBackup(dbPath, uploadDir, backupFilePath string) error {
	// 1. Try reading as a zip file
	zipReader, zipErr := zip.OpenReader(backupFilePath)
	if zipErr == nil {
		defer zipReader.Close()

		// Verify archive.db is present in zip
		var dbEntry *zip.File
		for _, f := range zipReader.File {
			cleanName := filepath.Clean(filepath.ToSlash(f.Name))
			if cleanName == "archive.db" || filepath.Base(cleanName) == "archive.db" {
				dbEntry = f
				break
			}
		}

		if dbEntry == nil {
			return fmt.Errorf("ملف النسخة الاحتياطية ZIP لا يحتوي على قاعدة البيانات archive.db")
		}

		// Stage into temp directory first to verify integrity
		stageDir, err := os.MkdirTemp("", "restore_stage_*")
		if err != nil {
			return fmt.Errorf("تعذر إنشاء مجلد مؤقت للاستعادة: %w", err)
		}
		defer os.RemoveAll(stageDir)

		stageDBPath := filepath.Join(stageDir, "archive.db")
		if err := extractZipEntry(dbEntry, stageDBPath); err != nil {
			return fmt.Errorf("فشل استخراج قاعدة البيانات من ملف النسخة الاحتياطية: %w", err)
		}

		// Verify database integrity
		testDB, err := sql.Open("sqlite", stageDBPath)
		if err != nil {
			return fmt.Errorf("قاعدة البيانات المستخرجة غير صالحة: %w", err)
		}
		var tableCount int
		err = testDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
		testDB.Close()
		if err != nil || tableCount == 0 {
			return fmt.Errorf("قاعدة البيانات في النسخة الاحتياطية تالفة أو فارغة")
		}

		// Extract uploads
		stageUploadsDir := filepath.Join(stageDir, "uploads")
		for _, f := range zipReader.File {
			slashName := filepath.ToSlash(f.Name)
			if strings.HasPrefix(slashName, "uploads/") && !f.FileInfo().IsDir() {
				relPath := strings.TrimPrefix(slashName, "uploads/")
				targetPath := filepath.Join(stageUploadsDir, relPath)
				if err := extractZipEntry(f, targetPath); err != nil {
					return fmt.Errorf("فشل استخراج الملف المرفق %s: %w", f.Name, err)
				}
			}
		}

		// Remove existing SQLite WAL and SHM files
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")

		// Overwrite target database with restored database
		if err := copyFile(stageDBPath, dbPath); err != nil {
			return fmt.Errorf("فشل نسخ قاعدة البيانات المستعادة إلى مسار العمل: %w", err)
		}

		// Copy restored uploads into uploadDir
		if info, err := os.Stat(stageUploadsDir); err == nil && info.IsDir() {
			_ = os.MkdirAll(uploadDir, 0755)
			_ = filepath.Walk(stageUploadsDir, func(path string, fileInfo os.FileInfo, walkErr error) error {
				if walkErr != nil || fileInfo.IsDir() {
					return walkErr
				}
				rel, err := filepath.Rel(stageUploadsDir, path)
				if err != nil {
					return err
				}
				dest := filepath.Join(uploadDir, rel)
				return copyFile(path, dest)
			})
		}

		return nil
	}

	// 2. If not a zip file, test as a direct SQLite .db file
	testDB, err := sql.Open("sqlite", backupFilePath)
	if err != nil {
		return fmt.Errorf("الملف المرفوع ليس ملف ZIP صالحاً أو قاعدة بيانات صالحة: %w", err)
	}
	var tableCount int
	err = testDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
	testDB.Close()
	if err != nil || tableCount == 0 {
		return fmt.Errorf("الملف المرفوع ليس ملف نسخ احتياطي صالح للأرشيف")
	}

	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	if err := copyFile(backupFilePath, dbPath); err != nil {
		return fmt.Errorf("فشل استبدال قاعدة البيانات: %w", err)
	}

	return nil
}

func extractZipEntry(f *zip.File, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func addFileToZip(zw *zip.Writer, srcPath, zipEntryName string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = zipEntryName
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, srcFile)
	return err
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
