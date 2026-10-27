// Modals, Forms & Native File Picker Integration
import { showAlert } from "./utils.js";
import { refreshStats } from "./stats.js";
import { loadDocuments } from "./documents.js";
import { openScannerStudio, discardScanSession } from "./scanner.js";

// Update form fields depending on doc_type (incoming vs outgoing)
export function updateFormDocType(form, docType) {
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

  form.querySelectorAll("[data-label-incoming]").forEach((el) => (el.hidden = isOutgoing));
  form.querySelectorAll("[data-label-outgoing]").forEach((el) => (el.hidden = !isOutgoing));
}

// Open Edit dialog
export function openEditDialog(doc) {
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
export function openDeleteDialog(id, subject) {
  const dialog = document.getElementById("delete-dialog");
  const form = document.getElementById("delete-form");

  form.querySelector('input[name="id"]').value = id;
  document.getElementById("delete-doc-title").textContent = `"${subject}"`;
  dialog.showModal();
}

// Setup Dialogs lifecycle and form submit listeners
export function setupDialogs() {
  // Setup dialog closing helpers and light dismiss
  document.querySelectorAll("dialog").forEach((dialog) => {
    dialog.querySelectorAll('button[value="cancel"], .preview-close-btn, .dialog-close').forEach((btn) => {
      btn.addEventListener("click", () => dialog.close());
    });

    // Light dismiss on backdrop click
    dialog.addEventListener("click", (e) => {
      const rect = dialog.getBoundingClientRect();
      const inDialog =
        rect.top <= e.clientY &&
        e.clientY <= rect.top + rect.height &&
        rect.left <= e.clientX &&
        e.clientX <= rect.left + rect.width;
      if (!inDialog) dialog.close();
    });
  });

  // Setup radio listeners for document types in forms
  document.querySelectorAll('input[name="doc_type"]').forEach((radio) => {
    radio.addEventListener("change", (e) => {
      const form = e.target.closest("form");
      if (form) updateFormDocType(form, e.target.value);
    });
  });

  // Open Add Dialog Button
  const btnOpenAdd = document.getElementById("btn-open-add");
  const addDialog = document.getElementById("add-dialog");
  const addForm = document.getElementById("add-form");
  const scannedPillContainer = document.getElementById("scanned-pill-container");
  const scannedPillText = document.getElementById("scanned-pill-text");

  const openAddHandler = () => {
    addForm.reset();
    updateFormDocType(addForm, "incoming");
    document.getElementById("selected-file-name").textContent = "لم يتم اختيار ملف";
    document.getElementById("add-source-path").value = "";
    if (scannedPillContainer) scannedPillContainer.style.display = "none";
    addDialog.showModal();
  };

  if (btnOpenAdd) btnOpenAdd.addEventListener("click", openAddHandler);
  document.querySelectorAll(".btn-empty-add").forEach((btn) => btn.addEventListener("click", openAddHandler));

  // Native File Picker Button
  const btnSelectFile = document.getElementById("btn-select-file");
  const selectedFileName = document.getElementById("selected-file-name");
  const addSourcePath = document.getElementById("add-source-path");

  if (btnSelectFile) {
    btnSelectFile.addEventListener("click", async () => {
      try {
        const path = await window.go.main.App.SelectDocumentFile();
        if (path) {
          addSourcePath.value = path;
          const filename = path.split(/[/\\]/).pop();
          selectedFileName.textContent = filename;
          if (scannedPillContainer) scannedPillContainer.style.display = "none";
        }
      } catch (err) {
        alert("خطأ أثناء اختيار الملف: " + err);
      }
    });
  }

  // WIA Scanner Studio Button
  const btnOpenScanner = document.getElementById("btn-open-scanner");
  if (btnOpenScanner) {
    btnOpenScanner.addEventListener("click", () => {
      openScannerStudio((pdfPath, pageCount) => {
        addSourcePath.value = pdfPath;
        const filename = pdfPath.split(/[/\\]/).pop();
        selectedFileName.textContent = filename;
        if (scannedPillContainer && scannedPillText) {
          scannedPillText.textContent = `تم مسح وتجميع ${pageCount} صفحة في مستند PDF جاهز للحفظ بالأرشيف`;
          scannedPillContainer.style.display = "flex";
        }
      });
    });
  }

  // Add Form Submit
  if (addForm) {
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

      if (payload.doc_type === "incoming" && payload.doc_date && payload.letter_date) {
        if (payload.doc_date < payload.letter_date) {
          alert("تاريخ ورود الكتاب (" + payload.doc_date + ") لا يمكن أن يكون قبل تاريخ الكتاب الوارد (" + payload.letter_date + ")");
          return;
        }
      }

      try {
        await window.go.main.App.CreateDocument(payload);
        addDialog.close();
        if (scannedPillContainer) scannedPillContainer.style.display = "none";
        showAlert("تم حفظ الوثيقة في الأرشيف بنجاح", "success");
        await refreshStats();
        await loadDocuments();
      } catch (err) {
        alert("خطأ أثناء حفظ الوثيقة: " + err);
      }
    });
  }

  // Edit Form Submit
  const editForm = document.getElementById("edit-form");
  const editDialog = document.getElementById("edit-dialog");
  if (editForm) {
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

      if (payload.doc_type === "incoming" && payload.doc_date && payload.letter_date) {
        if (payload.doc_date < payload.letter_date) {
          alert("تاريخ ورود الكتاب (" + payload.doc_date + ") لا يمكن أن يكون قبل تاريخ الكتاب الوارد (" + payload.letter_date + ")");
          return;
        }
      }

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
  }

  // Delete Form Submit
  const deleteForm = document.getElementById("delete-form");
  const deleteDialog = document.getElementById("delete-dialog");
  if (deleteForm) {
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
  }
}
