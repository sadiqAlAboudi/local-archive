// Local Archive client interactions (RTL / Arabic)
if ("serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {});
  });
}

document.addEventListener("DOMContentLoaded", () => {
  // Dialog Elements
  const addDialog = document.querySelector("dialog#add-dialog");
  const editDialog = document.querySelector("dialog#edit-dialog");
  const deleteDialog = document.querySelector("dialog#delete-dialog");

  // Open Add Dialog
  document.querySelectorAll('button[value="add"]').forEach(button => {
    button.addEventListener("click", () => {
      if (addDialog) {
        addDialog.showModal();
      }
    });
  });

  // Open Edit Dialog and populate fields
  document.querySelectorAll('button[value="edit"]').forEach(button => {
    button.addEventListener("click", () => {
      if (!editDialog) return;

      const id = button.getAttribute("data-id");
      const serialNumber = button.getAttribute("data-serial");
      const docDate = button.getAttribute("data-docdate");
      const department = button.getAttribute("data-dept");
      const letterNumber = button.getAttribute("data-letterno");
      const letterDate = button.getAttribute("data-letterdate");
      const subject = button.getAttribute("data-subject");

      const form = editDialog.querySelector("form");
      if (form) {
        form.querySelector('input[name="id"]').value = id || "";
        form.querySelector('input[name="serial_number"]').value = serialNumber || "";
        form.querySelector('input[name="doc_date"]').value = docDate || "";
        form.querySelector('input[name="department"]').value = department || "";
        form.querySelector('input[name="letter_number"]').value = letterNumber || "";
        form.querySelector('input[name="letter_date"]').value = letterDate || "";
        form.querySelector('textarea[name="subject"]').value = subject || "";
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
