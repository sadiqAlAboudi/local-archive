// Backup, Restore & Recovery Handlers
import { showAlert } from "./utils.js";
import { refreshStats } from "./stats.js";
import { loadDocuments } from "./documents.js";

// Handle Backup Creation
export async function handleCreateBackup() {
  try {
    const savedPath = await window.go.main.App.CreateBackup();
    if (savedPath) {
      sessionStorage.removeItem("dismiss_backup_reminder");
      showAlert(`تم حفظ النسخة الاحتياطية بنجاح في: ${savedPath}`, "success");
      await refreshStats();
    }
  } catch (err) {
    showAlert(`فشل إنشاء النسخة الاحتياطية: ${err}`, "error");
  }
}

// Handle Backup Restore
export async function handleRestoreBackup() {
  const confirmed = confirm(
    "تنبيه: ستؤدي عملية الاستعادة إلى استبدال قاعدة البيانات والملفات الحالية بالنسخة المحددة. هل ترغب بالاستمرار؟"
  );
  if (!confirmed) return;

  try {
    await window.go.main.App.RestoreBackup();
    showAlert("تمت استعادة النسخة الاحتياطية وتحديث بيانات الأرشيف بنجاح.", "success");
    await refreshStats();
    await loadDocuments();
  } catch (err) {
    showAlert(`فشل استعادة النسخة الاحتياطية: ${err}`, "error");
  }
}

// Handle Database Recovery
export async function handleRecoverDatabase() {
  const confirmed = confirm(
    "هل ترغب في فحص وإصلاح قاعدة البيانات؟\nسيقوم البرنامج بحفظ نسخة احتياطية من الملف الحالي تلقائياً واسترجاع كافة البيانات والوثائق الممكنة."
  );
  if (!confirmed) return;

  try {
    const result = await window.go.main.App.RecoverDatabase();
    if (result && result.success) {
      showAlert(result.message || "تم استرجاع وإصلاح قاعدة البيانات بنجاح", "success");
      await refreshStats();
      await loadDocuments();
    } else {
      showAlert("تعذر إصلاح قاعدة البيانات بشكل كامل", "error");
    }
  } catch (err) {
    showAlert(`فشل إصلاح قاعدة البيانات: ${err}`, "error");
  }
}

// Setup backup & repair header action buttons
export function setupBackupListeners() {
  const btnBackup = document.getElementById("btn-backup");
  if (btnBackup) btnBackup.addEventListener("click", handleCreateBackup);

  const btnRestore = document.getElementById("btn-restore");
  if (btnRestore) btnRestore.addEventListener("click", handleRestoreBackup);

  const btnRepair = document.getElementById("btn-repair-db");
  if (btnRepair) btnRepair.addEventListener("click", handleRecoverDatabase);
}
