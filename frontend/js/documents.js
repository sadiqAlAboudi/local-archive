// Documents List, Querying & Table Rendering
import { state, isDateFilterActive, resetFilterState } from "./state.js";
import { escapeHtml } from "./utils.js";
import { openPreview } from "./preview.js";
import { openEditDialog, openDeleteDialog } from "./dialogs.js";
import { resetDateFilterUI } from "./date-filter.js";

// Load and render documents list
export async function loadDocuments() {
  if (!window.go || !window.go.main || !window.go.main.App) return;

  const tbody = document.getElementById("docs-tbody");
  const emptyState = document.getElementById("empty-state");
  const table = document.getElementById("docs-table");
  const filterIndicator = document.getElementById("filter-indicator");
  const btnReset = document.getElementById("btn-reset-filter");
  const btnClearDate = document.getElementById("btn-clear-date");
  const rangeError = document.getElementById("date-range-error");

  const dateActive = isDateFilterActive();
  const hasFilter =
    state.searchQuery.trim() !== "" ||
    state.currentTypeFilter !== "all" ||
    dateActive;

  if (filterIndicator) {
    filterIndicator.style.display = hasFilter ? "inline-block" : "none";
    if (hasFilter) {
      let desc = "تم تطبيق التصفية والبحث";
      if (dateActive) {
        if (state.currentDateMode === "single") {
          desc = `تصفية: يوم ${state.filterStartDate}`;
        } else if (state.currentDateMode === "range") {
          if (state.filterStartDate && state.filterEndDate) {
            desc = `تصفية: من ${state.filterStartDate} إلى ${state.filterEndDate}`;
          } else if (state.filterStartDate) {
            desc = `تصفية: من ${state.filterStartDate}`;
          } else if (state.filterEndDate) {
            desc = `تصفية: لغاية ${state.filterEndDate}`;
          }
        }
      }
      filterIndicator.textContent = desc;
    }
  }

  if (btnReset) btnReset.style.display = hasFilter ? "inline-block" : "none";
  if (btnClearDate) btnClearDate.style.display = dateActive ? "inline-block" : "none";

  let queryStart = "";
  let queryEnd = "";

  if (state.currentDateMode === "single") {
    queryStart = state.filterStartDate;
    queryEnd = state.filterStartDate;
    if (rangeError) rangeError.style.display = "none";
  } else if (state.currentDateMode === "range") {
    queryStart = state.filterStartDate;
    queryEnd = state.filterEndDate;
    if (state.filterStartDate && state.filterEndDate && state.filterStartDate > state.filterEndDate) {
      if (rangeError) rangeError.style.display = "inline-block";
    } else {
      if (rangeError) rangeError.style.display = "none";
    }
  } else {
    if (rangeError) rangeError.style.display = "none";
  }

  try {
    const docs = await window.go.main.App.GetDocuments(
      state.searchQuery,
      state.currentTypeFilter,
      queryStart,
      queryEnd,
      state.filterDateField
    );
    tbody.innerHTML = "";

    if (!docs || docs.length === 0) {
      table.style.display = "none";
      emptyState.style.display = "flex";
      return;
    }

    table.style.display = "table";
    emptyState.style.display = "none";

    docs.forEach((doc) => {
      const isOutgoing = doc.doc_type === "outgoing";
      const typeArabic = isOutgoing ? "كتاب صادر" : "كتاب وارد";
      const fileBadge = doc.file_type === "pdf" ? "PDF" : "صورة";

      // Column 2 (رقم التسلسل): only incoming documents have an archive serial number
      const serialDisplay = !isOutgoing && doc.serial_number
        ? escapeHtml(doc.serial_number)
        : "—";

      // Column 3 (التاريخ): only incoming documents display receipt date here
      const dateDisplay = !isOutgoing && doc.doc_date
        ? `<time datetime="${doc.doc_date}">${doc.doc_date}</time>`
        : "—";

      // Column 5 (عدد وتاريخ الكتاب):
      // - Incoming: letter_number & letter_date
      // - Outgoing: issue_number & doc_date
      const bookNumber = isOutgoing ? doc.issue_number : doc.letter_number;
      const bookDate = isOutgoing ? doc.doc_date : doc.letter_date;
      const bookDisplay = bookNumber
        ? `${escapeHtml(bookNumber)}${bookDate ? `<br><small>${bookDate}</small>` : ""}`
        : "—";

      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td><mark value="${doc.doc_type}">${typeArabic}</mark></td>
        <td><strong>${serialDisplay}</strong></td>
        <td>${dateDisplay}</td>
        <td>${escapeHtml(doc.department)}</td>
        <td>${bookDisplay}</td>
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

// Setup search input and filter tabs listeners
export function setupSearchAndFilterListeners() {
  // Document Type Filter Tabs
  document.querySelectorAll('nav[data-filter="doc-type"] a').forEach((tab) => {
    tab.addEventListener("click", (e) => {
      e.preventDefault();
      document
        .querySelectorAll('nav[data-filter="doc-type"] a')
        .forEach((t) => t.removeAttribute("aria-current"));
      tab.setAttribute("aria-current", "page");
      state.currentTypeFilter = tab.getAttribute("data-type") || "all";
      loadDocuments();
    });
  });

  // Search Input live debounce filtering
  const searchInput = document.getElementById("search-input");
  let debounceTimeout = null;
  if (searchInput) {
    searchInput.addEventListener("input", (e) => {
      clearTimeout(debounceTimeout);
      debounceTimeout = setTimeout(() => {
        state.searchQuery = e.target.value;
        loadDocuments();
      }, 200);
    });
  }

  // Reset Filter Button
  const btnReset = document.getElementById("btn-reset-filter");
  if (btnReset) {
    btnReset.addEventListener("click", (e) => {
      e.preventDefault();
      if (searchInput) searchInput.value = "";
      resetFilterState();

      document.querySelectorAll('nav[data-filter="doc-type"] a').forEach((t) => {
        if (t.getAttribute("data-type") === "all") t.setAttribute("aria-current", "page");
        else t.removeAttribute("aria-current");
      });

      resetDateFilterUI();
      loadDocuments();
    });
  }
}
