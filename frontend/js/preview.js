// Document Preview Modal & Print Action
import { state } from "./state.js";
import { escapeHtml } from "./utils.js";

// Open preview modal
export function openPreview(doc) {
  state.currentPreviewDoc = doc;
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
    <div><dt>${isOutgoing ? "تاريخ الصدور" : "تاريخ ورود الكتاب"}</dt><dd>${doc.doc_date}</dd></div>
    <div><dt>${isOutgoing ? "الجهة الصادر إليها" : "اسم الدائرة"}</dt><dd>${escapeHtml(doc.department)}</dd></div>
    ${!isOutgoing && doc.letter_number ? `<div><dt>عدد الكتاب الوارد</dt><dd>${escapeHtml(doc.letter_number)}</dd></div>` : ""}
    ${!isOutgoing && doc.letter_date ? `<div><dt>تاريخ الكتاب الوارد</dt><dd>${escapeHtml(doc.letter_date)}</dd></div>` : ""}
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

// Setup preview controls (e.g., printing)
export function setupPreviewListeners() {
  const btnPrint = document.getElementById("btn-print-preview");
  if (btnPrint) {
    btnPrint.addEventListener("click", () => {
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
  }
}
