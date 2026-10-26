// In-App Auto-Update System
import { escapeHtml, showAlert } from "./utils.js";

let latestUpdateInfo = null;
let isUpdating = false;

// Check for updates from GitHub releases
export async function checkForUpdates(silent = true) {
  if (!window.go || !window.go.main || !window.go.main.App) return;

  try {
    const updateInfo = await window.go.main.App.CheckForUpdate();
    if (!updateInfo) return;

    latestUpdateInfo = updateInfo;

    if (updateInfo.available) {
      if (silent) {
        showUpdateBanner(updateInfo);
      } else {
        openUpdateDialog(updateInfo);
      }
    } else {
      if (!silent) {
        showAlert(`أنت تستخدم أحدث إصدار من التطبيق (${updateInfo.current_version})`, "info");
      }
    }
  } catch (err) {
    if (!silent) {
      showAlert("تعذر التحقق من وجود تحديثات: " + err, "error");
    } else {
      console.warn("Auto-update check failed:", err);
    }
  }
}

// Show update banner on main screen
export function showUpdateBanner(updateInfo) {
  const alertContainer = document.getElementById("alert-container");
  if (!alertContainer) return;

  const dismissKey = "dismiss_update_" + updateInfo.latest_version;
  if (sessionStorage.getItem(dismissKey) === "1") return;

  const existingBanner = document.getElementById("update-available-alert");
  if (existingBanner) existingBanner.remove();

  const banner = document.createElement("aside");
  banner.id = "update-available-alert";
  banner.setAttribute("role", "status");
  banner.setAttribute("data-banner", "update-available");
  banner.innerHTML = `
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"></path>
      <path d="M3 3v5h5"></path>
      <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"></path>
      <path d="M16 16h5v5"></path>
    </svg>
    <div>
      <strong>يوجد إصدار جديد متاح للأرشيف المحلي (${escapeHtml(updateInfo.latest_version)}):</strong>
      <span>${escapeHtml(updateInfo.release_title || "يتوفر تحديث جديد يحتوي على تحسينات وإصلاحات")}</span>
    </div>
    <button type="button" value="update-now" id="btn-banner-update">تحديث الآن</button>
    <button type="button" value="dismiss" id="btn-dismiss-update" title="إغلاق التنبيه">&times;</button>
  `;

  banner.querySelector("#btn-banner-update").addEventListener("click", () => {
    openUpdateDialog(updateInfo);
  });

  banner.querySelector("#btn-dismiss-update").addEventListener("click", () => {
    sessionStorage.setItem(dismissKey, "1");
    banner.remove();
  });

  alertContainer.prepend(banner);
}

// Open update details modal
export function openUpdateDialog(updateInfo) {
  if (!updateInfo) return;

  const dialog = document.getElementById("update-dialog");
  const newVerBadge = document.getElementById("update-new-version-badge");
  const currentVerLabel = document.getElementById("update-current-version-label");
  const releaseTitle = document.getElementById("update-release-title");
  const metaInfo = document.getElementById("update-meta-info");
  const progressContainer = document.getElementById("update-progress-container");
  const progressFill = document.getElementById("update-progress-fill");
  const progressPercent = document.getElementById("update-progress-percent");
  const progressStatus = document.getElementById("update-progress-status");
  const progressDetail = document.getElementById("update-progress-detail");
  const btnStart = document.getElementById("btn-start-update");
  const btnCancel = document.getElementById("btn-cancel-update");

  newVerBadge.textContent = `إصدار جديد: ${updateInfo.latest_version}`;
  currentVerLabel.textContent = `الإصدار الحالي: ${updateInfo.current_version}`;
  releaseTitle.textContent = updateInfo.release_title || `LocalArchive ${updateInfo.latest_version}`;

  let metaHtml = `<span>حجم التحديث: <strong>${escapeHtml(updateInfo.formatted_size || "")}</strong></span>`;
  if (updateInfo.published_at) {
    try {
      const pubDate = new Date(updateInfo.published_at).toLocaleDateString("ar-EG", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
      metaHtml += `<span>تاريخ الإصدار: <strong>${pubDate}</strong></span>`;
    } catch (_) {}
  }
  metaInfo.innerHTML = metaHtml;

  progressContainer.style.display = "none";
  progressFill.style.width = "0%";
  progressPercent.textContent = "0%";
  progressStatus.textContent = "جاري تنزيل التحديث...";
  progressDetail.textContent = "";

  btnStart.disabled = false;
  btnStart.style.display = "inline-flex";
  btnCancel.disabled = false;

  dialog.showModal();
}

// Start update download and execution
export async function startUpdate() {
  if (isUpdating) return;
  isUpdating = true;

  const btnStart = document.getElementById("btn-start-update");
  const btnCancel = document.getElementById("btn-cancel-update");
  const progressContainer = document.getElementById("update-progress-container");
  const progressFill = document.getElementById("update-progress-fill");
  const progressPercent = document.getElementById("update-progress-percent");
  const progressStatus = document.getElementById("update-progress-status");
  const progressDetail = document.getElementById("update-progress-detail");

  btnStart.disabled = true;
  btnCancel.disabled = true;
  progressContainer.style.display = "flex";
  progressFill.style.width = "0%";
  progressPercent.textContent = "0%";
  progressStatus.textContent = "جاري تنزيل التحديث إلى المجلد المؤقت (%TEMP%)...";
  progressDetail.textContent = "يرجى الانتظار، لا تغلق التطبيق...";

  try {
    await window.go.main.App.DownloadAndApplyUpdate();
  } catch (err) {
    isUpdating = false;
    btnStart.disabled = false;
    btnCancel.disabled = false;
    progressStatus.textContent = "خطأ أثناء التحديث";
    progressDetail.textContent = String(err);
    showAlert("حدث خطأ أثناء تنزيل أو تثبيت التحديث: " + err, "error");
  }
}

// Setup update event listeners
export function setupUpdateListeners() {
  const btnCheckUpdate = document.getElementById("btn-check-update");
  if (btnCheckUpdate) {
    btnCheckUpdate.addEventListener("click", () => checkForUpdates(false));
  }

  const btnStartUpdate = document.getElementById("btn-start-update");
  if (btnStartUpdate) {
    btnStartUpdate.addEventListener("click", startUpdate);
  }

  const updateDialog = document.getElementById("update-dialog");
  if (updateDialog) {
    updateDialog.querySelectorAll('button[value="cancel"]').forEach((btn) => {
      btn.addEventListener("click", () => {
        if (!isUpdating) updateDialog.close();
      });
    });
  }

  if (window.runtime && window.runtime.EventsOn) {
    window.runtime.EventsOn("update:progress", (data) => {
      const progressFill = document.getElementById("update-progress-fill");
      const progressPercent = document.getElementById("update-progress-percent");
      const progressDetail = document.getElementById("update-progress-detail");

      if (progressFill && data.percent !== undefined) {
        progressFill.style.width = `${data.percent.toFixed(1)}%`;
      }
      if (progressPercent && data.percent !== undefined) {
        progressPercent.textContent = `${Math.round(data.percent)}%`;
      }
      if (progressDetail && data.downloadedFormatted) {
        progressDetail.textContent = `تم تنزيل ${data.downloadedFormatted} من ${data.totalFormatted || ""}`;
      }
    });

    window.runtime.EventsOn("update:status", (status) => {
      const progressStatus = document.getElementById("update-progress-status");
      const progressDetail = document.getElementById("update-progress-detail");
      const progressFill = document.getElementById("update-progress-fill");
      const progressPercent = document.getElementById("update-progress-percent");

      if (status === "downloading") {
        if (progressStatus) progressStatus.textContent = "جاري تنزيل ملف التحديث...";
      } else if (status === "installing") {
        if (progressFill) progressFill.style.width = "100%";
        if (progressPercent) progressPercent.textContent = "100%";
        if (progressStatus)
          progressStatus.textContent =
            "تم التنزيل بنجاح! جاري تشغيل المثبت الصامت وإغلاق التطبيق...";
        if (progressDetail)
          progressDetail.textContent =
            "سيتم إغلاق التطبيق الآن ليتمكن المثبت من تحديث واستبدال الملفات.";
      } else if (status === "error") {
        if (progressStatus) progressStatus.textContent = "فشل تثبيت التحديث";
      }
    });
  }
}
