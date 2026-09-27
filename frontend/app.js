// Local Archive - Wails Desktop Client Application

let currentTypeFilter = "all";
let searchQuery = "";
let currentPreviewDoc = null;

// Helper to escape HTML characters
function escapeHtml(str) {
  if (!str) return "";
  return String(str)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

// Show alert banner
function showAlert(message, type = "info") {
  const container = document.getElementById("alert-container");
  if (!container) return;

  const aside = document.createElement("aside");
  aside.setAttribute("role", type === "error" ? "alert" : "status");
  aside.innerHTML = `
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      ${type === "error" 
        ? '<circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line>' 
        : '<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline>'}
    </svg>
    <span>${escapeHtml(message)}</span>
    <button type="button" style="margin-right: auto; background: none; border: none; cursor: pointer; font-size: 1.2rem; color: inherit;">&times;</button>
  `;

  aside.querySelector("button").addEventListener("click", () => aside.remove());
  container.appendChild(aside);

  if (type !== "error") {
    setTimeout(() => {
      if (aside.parentElement) aside.remove();
    }, 6000);
  }
}

// Update form fields depending on doc_type (incoming vs outgoing)
function updateFormDocType(form, docType) {
  if (!form) return;
  const isOutgoing = docType === "outgoing";

  const radio = form.querySelector(`input[name="doc_type"][value="${docType}"]`);
  if (radio) radio.checked = true;

  const serialLabel = form.querySelector('label[data-field="serial_number"]');
  if (serialLabel) {
    serialLabel.hidden = isOutgoing;
    const input = serialLabel.querySelector("input");
    if (input) input.required = !isOutgoing;
  }

  const letterNoLabel = form.querySelector('label[data-field="letter_number"]');
  if (letterNoLabel) {
    letterNoLabel.hidden = isOutgoing;
    const input = letterNoLabel.querySelector("input");
    if (input) input.required = !isOutgoing;
  }

  const letterDateLabel = form.querySelector('label[data-field="letter_date"]');
  if (letterDateLabel) {
    letterDateLabel.hidden = isOutgoing;
    const input = letterDateLabel.querySelector("input");
    if (input) input.required = !isOutgoing;
  }

  const issueLabel = form.querySelector('label[data-field="issue_number"]');
  if (issueLabel) {
    issueLabel.hidden = !isOutgoing;
    const input = issueLabel.querySelector("input");
    if (input) input.required = isOutgoing;
  }

  form.querySelectorAll("[data-label-incoming]").forEach(el => el.hidden = isOutgoing);
  form.querySelectorAll("[data-label-outgoing]").forEach(el => el.hidden = !isOutgoing);
}

// Refresh statistics and backup reminders
async function refreshStats() {
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
            ${stats.LastBackupDays > 0 
              ? `مضى ${stats.LastBackupDays} أيام على آخر عملية نسخ احتياطي. للحفاظ على وثائقك وسجلاتك، يُرجى حفظ نسخة احتياطية الآن.`
              : `لم تقم بأخذ نسخة احتياطية للأرشيف منذ أكثر من 5 أيام. لحماية بياناتك ومستنداتك، يُنصح بحفظ نسخة احتياطية شاملة الآن.`}
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

// Load and render documents list
async function loadDocuments() {
  if (!window.go || !window.go.main || !window.go.main.App) return;

  const tbody = document.getElementById("docs-tbody");
  const emptyState = document.getElementById("empty-state");
  const table = document.getElementById("docs-table");
  const filterIndicator = document.getElementById("filter-indicator");
  const btnReset = document.getElementById("btn-reset-filter");

  const hasFilter = searchQuery.trim() !== "" || currentTypeFilter !== "all";
  if (filterIndicator) filterIndicator.style.display = hasFilter ? "inline-block" : "none";
  if (btnReset) btnReset.style.display = hasFilter ? "inline-block" : "none";

  try {
    const docs = await window.go.main.App.GetDocuments(searchQuery, currentTypeFilter);
    tbody.innerHTML = "";

    if (!docs || docs.length === 0) {
      table.style.display = "none";
      emptyState.style.display = "flex";
      return;
    }

    table.style.display = "table";
    emptyState.style.display = "none";

    docs.forEach(doc => {
      const isOutgoing = doc.doc_type === "outgoing";
      const refNum = isOutgoing ? doc.issue_number : doc.serial_number;
      const typeArabic = isOutgoing ? "كتاب صادر" : "كتاب وارد";
      const fileBadge = doc.file_type === "pdf" ? "PDF" : "صورة";

      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td><mark value="${doc.doc_type}">${typeArabic}</mark></td>
        <td><strong>${escapeHtml(refNum)}</strong></td>
        <td><time datetime="${doc.doc_date}">${doc.doc_date}</time></td>
        <td>${escapeHtml(doc.department)}</td>
        <td>
          ${!isOutgoing && doc.letter_number 
            ? `${escapeHtml(doc.letter_number)}${doc.letter_date ? `<br><small>${doc.letter_date}</small>` : ""}` 
            : "—"}
        </td>
        <td>
          <a href="#" value="title-link" class="preview-link" data-id="${doc.id}">
            ${escapeHtml(doc.subject)}
          </a>
        </td>
        <td><mark>${fileBadge}</mark></td>
        <td>
          <menu>
            <li>
              <button type="button" value="preview" class="action-preview" data-id="${doc.id}" title="معاينة الوثيقة">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                  <circle cx="12" cy="12" r="3"></circle>
                </svg>
                معاينة
              </button>
            </li>
            <li>
              <button type="button" value="edit" class="action-edit" data-id="${doc.id}" title="تعديل">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                </svg>
                تعديل
              </button>
            </li>
            <li>
              <button type="button" value="delete" class="action-delete" data-id="${doc.id}" data-subject="${escapeHtml(doc.subject)}" title="حذف">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="3 6 5 6 21 6"></polyline>
                  <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                </svg>
                حذف
              </button>
            </li>
          </menu>
        </td>
      `;

      tr.querySelector(".preview-link").addEventListener("click", (e) => {
        e.preventDefault();
        openPreview(doc);
      });
      tr.querySelector(".action-preview").addEventListener("click", () => openPreview(doc));
      tr.querySelector(".action-edit").addEventListener("click", () => openEditDialog(doc));
      tr.querySelector(".action-delete").addEventListener("click", () => openDeleteDialog(doc.id, doc.subject));

      tbody.appendChild(tr);
    });
  } catch (err) {
    console.error("Failed to load documents:", err);
  }
}

// Open preview modal
function openPreview(doc) {
  currentPreviewDoc = doc;
  const modal = document.getElementById("preview-dialog");
  const title = document.getElementById("preview-title");
  const dl = document.getElementById("preview-dl");
  const iframe = document.getElementById("preview-iframe");
  const figure = document.getElementById("preview-figure");
  const img = document.getElementById("preview-img");

  title.textContent = doc.subject;

  const isOutgoing = doc.doc_type === "outgoing";
  dl.innerHTML = `
    <div><dt>نوع الوثيقة</dt><dd><mark value="${doc.doc_type}">${isOutgoing ? "كتاب صادر" : "كتاب وارد"}</mark></dd></div>
    <div><dt>${isOutgoing ? "العدد" : "رقم التسلسل"}</dt><dd><strong>${escapeHtml(isOutgoing ? doc.issue_number : doc.serial_number)}</strong></dd></div>
    <div><dt>${isOutgoing ? "تاريخ الصدور" : "تاريخ التسجيل"}</dt><dd>${doc.doc_date}</dd></div>
    <div><dt>${isOutgoing ? "الجهة الصادر إليها" : "اسم الدائرة"}</dt><dd>${escapeHtml(doc.department)}</dd></div>
    ${!isOutgoing && doc.letter_number ? `<div><dt>رقم كتاب الجهة</dt><dd>${escapeHtml(doc.letter_number)}</dd></div>` : ""}
    ${!isOutgoing && doc.letter_date ? `<div><dt>تاريخ كتاب الجهة</dt><dd>${escapeHtml(doc.letter_date)}</dd></div>` : ""}
    <div><dt>الموضوع</dt><dd>${escapeHtml(doc.subject)}</dd></div>
    <div><dt>الملف الأصلي</dt><dd>${escapeHtml(doc.original_filename)}</dd></div>
  `;

  const fileUrl = `/view-file?file=${encodeURIComponent(doc.filename)}`;
  if (doc.file_type === "pdf") {
    figure.style.display = "none";
    img.src = "";
    iframe.src = fileUrl;
    iframe.style.display = "block";
  } else {
    iframe.style.display = "none";
    iframe.src = "";
    img.src = fileUrl;
    figure.style.display = "flex";
  }

  modal.showModal();
}

// Open Edit dialog
function openEditDialog(doc) {
  const dialog = document.getElementById("edit-dialog");
  const form = document.getElementById("edit-form");

  form.querySelector('input[name="id"]').value = doc.id;
  updateFormDocType(form, doc.doc_type);

  form.querySelector('input[name="serial_number"]').value = doc.serial_number || "";
  form.querySelector('input[name="issue_number"]').value = doc.issue_number || "";
  form.querySelector('input[name="doc_date"]').value = doc.doc_date || "";
  form.querySelector('input[name="department"]').value = doc.department || "";
  form.querySelector('input[name="letter_number"]').value = doc.letter_number || "";
  form.querySelector('input[name="letter_date"]').value = doc.letter_date || "";
  form.querySelector('textarea[name="subject"]').value = doc.subject || "";

  dialog.showModal();
}

// Open Delete dialog
function openDeleteDialog(id, subject) {
  const dialog = document.getElementById("delete-dialog");
  const form = document.getElementById("delete-form");

  form.querySelector('input[name="id"]').value = id;
  document.getElementById("delete-doc-title").textContent = `"${subject}"`;
  dialog.showModal();
}

// Handle Backup Creation
async function handleCreateBackup() {
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
async function handleRestoreBackup() {
  const confirmed = confirm("تنبيه: ستؤدي عملية الاستعادة إلى استبدال قاعدة البيانات والملفات الحالية بالنسخة المحددة. هل ترغب بالاستمرار؟");
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

// Initialize Application
document.addEventListener("DOMContentLoaded", () => {
  // Setup dialog closing helpers
  document.querySelectorAll("dialog").forEach(dialog => {
    dialog.querySelectorAll('button[value="cancel"], .preview-close-btn').forEach(btn => {
      btn.addEventListener("click", () => dialog.close());
    });

    // Light dismiss on backdrop click
    dialog.addEventListener("click", (e) => {
      const rect = dialog.getBoundingClientRect();
      const inDialog = (
        rect.top <= e.clientY &&
        e.clientY <= rect.top + rect.height &&
        rect.left <= e.clientX &&
        e.clientX <= rect.left + rect.width
      );
      if (!inDialog) dialog.close();
    });
  });

  // Setup radio listeners for document types
  document.querySelectorAll('input[name="doc_type"]').forEach(radio => {
    radio.addEventListener("change", (e) => {
      const form = e.target.closest("form");
      if (form) updateFormDocType(form, e.target.value);
    });
  });

  // Open Add Dialog Button
  const btnOpenAdd = document.getElementById("btn-open-add");
  const addDialog = document.getElementById("add-dialog");
  const addForm = document.getElementById("add-form");

  const openAddHandler = () => {
    addForm.reset();
    updateFormDocType(addForm, "incoming");
    document.getElementById("selected-file-name").textContent = "لم يتم اختيار ملف";
    document.getElementById("add-source-path").value = "";
    addDialog.showModal();
  };

  btnOpenAdd.addEventListener("click", openAddHandler);
  document.querySelectorAll(".btn-empty-add").forEach(btn => btn.addEventListener("click", openAddHandler));

  // Native File Picker Button
  const btnSelectFile = document.getElementById("btn-select-file");
  const selectedFileName = document.getElementById("selected-file-name");
  const addSourcePath = document.getElementById("add-source-path");

  btnSelectFile.addEventListener("click", async () => {
    try {
      const path = await window.go.main.App.SelectDocumentFile();
      if (path) {
        addSourcePath.value = path;
        const filename = path.split(/[/\\]/).pop();
        selectedFileName.textContent = filename;
      }
    } catch (err) {
      alert("خطأ أثناء اختيار الملف: " + err);
    }
  });

  // Add Form Submit
  addForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (!addSourcePath.value) {
      alert("يرجى اختيار ملف الوثيقة من الحاسوب");
      return;
    }

    const formData = new FormData(addForm);
    const payload = {
      doc_type: formData.get("doc_type"),
      serial_number: formData.get("serial_number") || "",
      issue_number: formData.get("issue_number") || "",
      doc_date: formData.get("doc_date") || "",
      department: formData.get("department") || "",
      letter_number: formData.get("letter_number") || "",
      letter_date: formData.get("letter_date") || "",
      subject: formData.get("subject") || "",
      source_path: addSourcePath.value,
    };

    try {
      await window.go.main.App.CreateDocument(payload);
      addDialog.close();
      showAlert("تم حفظ الوثيقة في الأرشيف بنجاح", "success");
      await refreshStats();
      await loadDocuments();
    } catch (err) {
      alert("خطأ أثناء حفظ الوثيقة: " + err);
    }
  });

  // Edit Form Submit
  const editForm = document.getElementById("edit-form");
  const editDialog = document.getElementById("edit-dialog");
  editForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    const formData = new FormData(editForm);
    const payload = {
      id: parseInt(formData.get("id"), 10),
      doc_type: formData.get("doc_type"),
      serial_number: formData.get("serial_number") || "",
      issue_number: formData.get("issue_number") || "",
      doc_date: formData.get("doc_date") || "",
      department: formData.get("department") || "",
      letter_number: formData.get("letter_number") || "",
      letter_date: formData.get("letter_date") || "",
      subject: formData.get("subject") || "",
    };

    try {
      await window.go.main.App.UpdateDocument(payload);
      editDialog.close();
      showAlert("تم حفظ التعديلات بنجاح", "success");
      await refreshStats();
      await loadDocuments();
    } catch (err) {
      alert("خطأ أثناء تحديث الوثيقة: " + err);
    }
  });

  // Delete Form Submit
  const deleteForm = document.getElementById("delete-form");
  const deleteDialog = document.getElementById("delete-dialog");
  deleteForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    const id = parseInt(deleteForm.querySelector('input[name="id"]').value, 10);
    try {
      await window.go.main.App.DeleteDocument(id);
      deleteDialog.close();
      showAlert("تم حذف الوثيقة بنجاح", "success");
      await refreshStats();
      await loadDocuments();
    } catch (err) {
      alert("خطأ أثناء حذف الوثيقة: " + err);
    }
  });

  // Filter tabs
  document.querySelectorAll('nav[data-filter="doc-type"] a').forEach(tab => {
    tab.addEventListener("click", (e) => {
      e.preventDefault();
      document.querySelectorAll('nav[data-filter="doc-type"] a').forEach(t => t.removeAttribute("aria-current"));
      tab.setAttribute("aria-current", "page");
      currentTypeFilter = tab.getAttribute("data-type") || "all";
      loadDocuments();
    });
  });

  // Search input live filtering
  const searchInput = document.getElementById("search-input");
  let debounceTimeout = null;
  searchInput.addEventListener("input", (e) => {
    clearTimeout(debounceTimeout);
    debounceTimeout = setTimeout(() => {
      searchQuery = e.target.value;
      loadDocuments();
    }, 200);
  });

  // Reset filter
  document.getElementById("btn-reset-filter").addEventListener("click", (e) => {
    e.preventDefault();
    searchInput.value = "";
    searchQuery = "";
    currentTypeFilter = "all";
    document.querySelectorAll('nav[data-filter="doc-type"] a').forEach(t => {
      if (t.getAttribute("data-type") === "all") t.setAttribute("aria-current", "page");
      else t.removeAttribute("aria-current");
    });
    loadDocuments();
  });

  // Backup & Restore header buttons
  document.getElementById("btn-backup").addEventListener("click", handleCreateBackup);
  document.getElementById("btn-restore").addEventListener("click", handleRestoreBackup);

  // Preview dialog controls
  document.getElementById("btn-print-preview").addEventListener("click", () => {
    const iframe = document.getElementById("preview-iframe");
    if (iframe && iframe.style.display !== "none" && iframe.contentWindow) {
      try {
        iframe.contentWindow.focus();
        iframe.contentWindow.print();
        return;
      } catch (_) {}
    }
    window.print();
  });

  // Update controls
  setupUpdateEventListeners();

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
    updateDialog.querySelectorAll('button[value="cancel"]').forEach(btn => {
      btn.addEventListener("click", () => {
        if (!isUpdating) updateDialog.close();
      });
    });
  }

  // Initial load
  refreshStats();
  loadDocuments();

  // Check for updates silently on startup after short delay
  setTimeout(() => {
    checkForUpdates(true);
  }, 1500);
});

// --- Auto-Update System ---
let latestUpdateInfo = null;
let isUpdating = false;

// Check for updates from GitHub releases
async function checkForUpdates(silent = true) {
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
function showUpdateBanner(updateInfo) {
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
function openUpdateDialog(updateInfo) {
  if (!updateInfo) return;

  const dialog = document.getElementById("update-dialog");
  const newVerBadge = document.getElementById("update-new-version-badge");
  const currentVerLabel = document.getElementById("update-current-version-label");
  const releaseTitle = document.getElementById("update-release-title");
  const metaInfo = document.getElementById("update-meta-info");
  const notesContent = document.getElementById("update-notes-content");
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
      const pubDate = new Date(updateInfo.published_at).toLocaleDateString("ar-EG", { year: "numeric", month: "long", day: "numeric" });
      metaHtml += `<span>تاريخ الإصدار: <strong>${pubDate}</strong></span>`;
    } catch (_) {}
  }
  metaInfo.innerHTML = metaHtml;

  notesContent.textContent = updateInfo.release_notes || "تحسينات وإصلاحات عامة في النظام والاستقرار.";

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
async function startUpdate() {
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

// Attach Wails runtime event listeners for download progress and status
function setupUpdateEventListeners() {
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
        if (progressStatus) progressStatus.textContent = "تم التنزيل بنجاح! جاري تشغيل المثبت الصامت وإغلاق التطبيق...";
        if (progressDetail) progressDetail.textContent = "سيتم إغلاق التطبيق الآن ليتمكن المثبت من تحديث واستبدال الملفات.";
      } else if (status === "error") {
        if (progressStatus) progressStatus.textContent = "فشل تثبيت التحديث";
      }
    });
  }
}

