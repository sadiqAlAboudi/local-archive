// Statistics & Backup Reminders
import { handleCreateBackup } from "./backup.js";

export async function refreshStats() {
  if (!window.go || !window.go.main || !window.go.main.App) return;

  try {
    const stats = await window.go.main.App.GetStats();
    document.getElementById("stat-total-docs").textContent = stats.TotalDocs;
    document.getElementById("stat-incoming-docs").textContent = stats.TotalIncoming;
    document.getElementById("stat-outgoing-docs").textContent = stats.TotalOutgoing;
    document.getElementById("stat-storage-used").textContent = stats.StorageUsed;

    document.getElementById("filter-count-all").textContent = stats.TotalDocs;
    document.getElementById("filter-count-incoming").textContent = stats.TotalIncoming;
    document.getElementById("filter-count-outgoing").textContent = stats.TotalOutgoing;

    const alertContainer = document.getElementById("alert-container");
    const existingReminder = document.getElementById("backup-reminder-alert");
    if (existingReminder) existingReminder.remove();

    if (stats.ShowBackupReminder && sessionStorage.getItem("dismiss_backup_reminder") !== "1") {
      const banner = document.createElement("aside");
      banner.id = "backup-reminder-alert";
      banner.setAttribute("role", "note");
      banner.setAttribute("data-banner", "backup-reminder");
      banner.innerHTML = `
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
          <line x1="12" y1="9" x2="12" y2="13"></line>
          <line x1="12" y1="17" x2="12.01" y2="17"></line>
        </svg>
        <div>
          <strong>تذكير دوري بالنسخ الاحتياطي:</strong>
          <span>
            ${
              stats.LastBackupDays > 0
                ? `مضى ${stats.LastBackupDays} أيام على آخر عملية نسخ احتياطي. للحفاظ على وثائقك وسجلاتك، يُرجى حفظ نسخة احتياطية الآن.`
                : `لم تقم بأخذ نسخة احتياطية للأرشيف منذ أكثر من 5 أيام. لحماية بياناتك ومستنداتك، يُنصح بحفظ نسخة احتياطية شاملة الآن.`
            }
          </span>
        </div>
        <a href="#" value="backup-now" id="btn-backup-now">حفظ نسخة احتياطية الآن</a>
        <button type="button" value="dismiss" id="btn-dismiss-backup" title="إغلاق التنبيه">&times;</button>
      `;

      banner.querySelector("#btn-backup-now").addEventListener("click", (e) => {
        e.preventDefault();
        handleCreateBackup();
      });
      banner.querySelector("#btn-dismiss-backup").addEventListener("click", () => {
        sessionStorage.setItem("dismiss_backup_reminder", "1");
        banner.remove();
      });
      alertContainer.prepend(banner);
    }
  } catch (err) {
    console.error("Failed to load statistics:", err);
  }
}
