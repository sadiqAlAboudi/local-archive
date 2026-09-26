// Local Archive client interactions (RTL / Arabic)
if ("serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {});
  });
}

function updateFormDocType(form, docType) {
  if (!form) return;
  const isOutgoing = docType === "outgoing";

  // Check the corresponding radio button
  const radio = form.querySelector(`input[name="doc_type"][value="${docType}"]`);
  if (radio) {
    radio.checked = true;
  }

  // Incoming specific fields: serial_number, letter_number, letter_date
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

  // Outgoing specific field: issue_number
  const issueLabel = form.querySelector('label[data-field="issue_number"]');
  if (issueLabel) {
    issueLabel.hidden = !isOutgoing;
    const input = issueLabel.querySelector("input");
    if (input) input.required = isOutgoing;
  }

  // Toggle dynamic labels for shared fields
  form.querySelectorAll("[data-label-incoming]").forEach(el => {
    el.hidden = isOutgoing;
  });
  form.querySelectorAll("[data-label-outgoing]").forEach(el => {
    el.hidden = !isOutgoing;
  });
}

document.addEventListener("DOMContentLoaded", () => {
  // Dialog Elements
  const addDialog = document.querySelector("dialog#add-dialog");
  const editDialog = document.querySelector("dialog#edit-dialog");
  const deleteDialog = document.querySelector("dialog#delete-dialog");
  const restoreDialog = document.querySelector("dialog#restore-dialog");

  // Open Restore Dialog
  document.querySelectorAll('button[value="restore"]').forEach(button => {
    button.addEventListener("click", () => {
      if (restoreDialog) {
        restoreDialog.showModal();
      }
    });
  });

  // Open restore dialog if hash is #restore
  if (window.location.hash === "#restore" && restoreDialog) {
    restoreDialog.showModal();
    if (window.history && window.history.replaceState) {
      window.history.replaceState(null, "", window.location.pathname + window.location.search);
    }
  }

  // Setup radio change listeners for all doc_type radios
  document.querySelectorAll('input[name="doc_type"]').forEach(radio => {
    radio.addEventListener("change", (e) => {
      const form = e.target.closest("form");
      if (form) {
        updateFormDocType(form, e.target.value);
      }
    });
  });

  // Open Add Dialog
  document.querySelectorAll('button[value="add"]').forEach(button => {
    button.addEventListener("click", () => {
      if (addDialog) {
        const form = addDialog.querySelector("form");
        if (form) {
          form.reset();
          updateFormDocType(form, "incoming");
        }
        addDialog.showModal();
      }
    });
  });

  // Open Edit Dialog and populate fields
  document.querySelectorAll('button[value="edit"]').forEach(button => {
    button.addEventListener("click", () => {
      if (!editDialog) return;

      const id = button.getAttribute("data-id");
      const docType = button.getAttribute("data-type") || "incoming";
      const serialNumber = button.getAttribute("data-serial");
      const issueNumber = button.getAttribute("data-issue");
      const docDate = button.getAttribute("data-docdate");
      const department = button.getAttribute("data-dept");
      const letterNumber = button.getAttribute("data-letterno");
      const letterDate = button.getAttribute("data-letterdate");
      const subject = button.getAttribute("data-subject");

      const form = editDialog.querySelector("form");
      if (form) {
        form.querySelector('input[name="id"]').value = id || "";
        updateFormDocType(form, docType);

        const serialInput = form.querySelector('input[name="serial_number"]');
        if (serialInput) serialInput.value = serialNumber || "";

        const issueInput = form.querySelector('input[name="issue_number"]');
        if (issueInput) issueInput.value = issueNumber || "";

        const docDateInput = form.querySelector('input[name="doc_date"]');
        if (docDateInput) docDateInput.value = docDate || "";

        const deptInput = form.querySelector('input[name="department"]');
        if (deptInput) deptInput.value = department || "";

        const letterNoInput = form.querySelector('input[name="letter_number"]');
        if (letterNoInput) letterNoInput.value = letterNumber || "";

        const letterDateInput = form.querySelector('input[name="letter_date"]');
        if (letterDateInput) letterDateInput.value = letterDate || "";

        const subjectInput = form.querySelector('textarea[name="subject"]');
        if (subjectInput) subjectInput.value = subject || "";
      }

      editDialog.showModal();
    });
  });

  // Open Delete Dialog and populate confirmation
  document.querySelectorAll('button[value="delete"]').forEach(button => {
    button.addEventListener("click", () => {
      if (!deleteDialog) return;

      const id = button.getAttribute("data-id");
      const subject = button.getAttribute("data-subject");

      const form = deleteDialog.querySelector("form");
      if (form) {
        form.querySelector('input[name="id"]').value = id || "";
        const titleSpan = deleteDialog.querySelector("strong");
        if (titleSpan) {
          titleSpan.textContent = `"${subject}"`;
        }
      }

      deleteDialog.showModal();
    });
  });

  // Close dialogs when cancel button is clicked
  document.querySelectorAll('dialog button[value="cancel"]').forEach(button => {
    button.addEventListener("click", (e) => {
      const dialog = e.target.closest("dialog");
      if (dialog) {
        dialog.close();
      }
    });
  });

  // Light dismiss: close dialog when clicking outside (on backdrop)
  document.querySelectorAll("dialog").forEach(dialog => {
    dialog.addEventListener("click", (e) => {
      const rect = dialog.getBoundingClientRect();
      const isInDialog = (
        rect.top <= e.clientY &&
        e.clientY <= rect.top + rect.height &&
        rect.left <= e.clientX &&
        e.clientX <= rect.left + rect.width
      );
      if (!isInDialog) {
        dialog.close();
      }
    });
  });

  // Print button functionality
  const printButton = document.querySelector('button[value="print"]');
  if (printButton) {
    printButton.addEventListener("click", () => {
      const iframe = document.querySelector("iframe");
      if (iframe && iframe.contentWindow) {
        try {
          iframe.contentWindow.focus();
          iframe.contentWindow.print();
          return;
        } catch (_) {
          // Fall back to window print
        }
      }
      window.print();
    });
  }

  // Live table search filtering on input (supports multi-term search)
  const searchInput = document.querySelector('input[type="search"]');
  const tableBody = document.querySelector("tbody");
  if (searchInput && tableBody) {
    searchInput.addEventListener("input", (e) => {
      const terms = e.target.value.toLowerCase().trim().split(/\s+/).filter(Boolean);
      const rows = tableBody.querySelectorAll("tr");

      rows.forEach(row => {
        if (terms.length === 0) {
          row.style.display = "";
          return;
        }
        const text = row.innerText.toLowerCase();
        const matches = terms.every(term => text.includes(term));
        row.style.display = matches ? "" : "none";
      });
    });
  }
});
