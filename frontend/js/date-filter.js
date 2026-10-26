// Date Filter Toolbar & Presets
import { state } from "./state.js";
import { formatDateIso } from "./utils.js";
import { loadDocuments } from "./documents.js";

export function setupDateFilterListeners() {
  const panel = document.getElementById("date-filter-panel");
  const viewSingle = document.getElementById("view-single-date");
  const viewRange = document.getElementById("view-range-date");
  const inputSingle = document.getElementById("filter-single-date");
  const inputStart = document.getElementById("filter-start-date");
  const inputEnd = document.getElementById("filter-end-date");
  const selectField = document.getElementById("filter-date-field");
  const btnClearDate = document.getElementById("btn-clear-date");

  function setMode(mode) {
    state.currentDateMode = mode;
    document.querySelectorAll(".date-tab-btn").forEach((btn) => {
      const active = btn.getAttribute("data-mode") === mode;
      btn.classList.toggle("active", active);
      btn.setAttribute("aria-selected", active ? "true" : "false");
    });

    if (mode === "all") {
      if (panel) panel.style.display = "none";
      if (viewSingle) viewSingle.style.display = "none";
      if (viewRange) viewRange.style.display = "none";
      state.filterStartDate = "";
      state.filterEndDate = "";
      if (inputSingle) inputSingle.value = "";
      if (inputStart) inputStart.value = "";
      if (inputEnd) inputEnd.value = "";
      loadDocuments();
    } else if (mode === "single") {
      if (panel) panel.style.display = "flex";
      if (viewSingle) viewSingle.style.display = "flex";
      if (viewRange) viewRange.style.display = "none";
      state.filterEndDate = "";
      if (inputSingle && !inputSingle.value && inputStart && inputStart.value) {
        inputSingle.value = inputStart.value;
      }
      state.filterStartDate = inputSingle ? inputSingle.value : "";
      loadDocuments();
    } else if (mode === "range") {
      if (panel) panel.style.display = "flex";
      if (viewSingle) viewSingle.style.display = "none";
      if (viewRange) viewRange.style.display = "flex";
      if (inputStart && !inputStart.value && inputSingle && inputSingle.value) {
        inputStart.value = inputSingle.value;
      }
      state.filterStartDate = inputStart ? inputStart.value : "";
      state.filterEndDate = inputEnd ? inputEnd.value : "";
      loadDocuments();
    }
  }

  document.querySelectorAll(".date-tab-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
      setMode(btn.getAttribute("data-mode"));
    });
  });

  if (inputSingle) {
    inputSingle.addEventListener("change", (e) => {
      state.filterStartDate = e.target.value;
      loadDocuments();
    });
  }

  if (inputStart) {
    inputStart.addEventListener("change", (e) => {
      state.filterStartDate = e.target.value;
      loadDocuments();
    });
  }

  if (inputEnd) {
    inputEnd.addEventListener("change", (e) => {
      state.filterEndDate = e.target.value;
      loadDocuments();
    });
  }

  if (selectField) {
    selectField.addEventListener("change", (e) => {
      state.filterDateField = e.target.value;
      loadDocuments();
    });
  }

  if (btnClearDate) {
    btnClearDate.addEventListener("click", () => {
      state.filterStartDate = "";
      state.filterEndDate = "";
      if (inputSingle) inputSingle.value = "";
      if (inputStart) inputStart.value = "";
      if (inputEnd) inputEnd.value = "";
      loadDocuments();
    });
  }

  document.querySelectorAll(".btn-date-preset").forEach((btn) => {
    btn.addEventListener("click", () => {
      const preset = btn.getAttribute("data-preset");
      const today = new Date();

      if (preset === "today") {
        const todayStr = formatDateIso(today);
        if (state.currentDateMode === "single") {
          if (inputSingle) inputSingle.value = todayStr;
          state.filterStartDate = todayStr;
        } else if (state.currentDateMode === "range") {
          if (inputStart) inputStart.value = todayStr;
          if (inputEnd) inputEnd.value = todayStr;
          state.filterStartDate = todayStr;
          state.filterEndDate = todayStr;
        }
        loadDocuments();
      } else if (preset === "yesterday") {
        const yesterday = new Date(today);
        yesterday.setDate(yesterday.getDate() - 1);
        const yestStr = formatDateIso(yesterday);
        if (state.currentDateMode === "single") {
          if (inputSingle) inputSingle.value = yestStr;
          state.filterStartDate = yestStr;
        } else if (state.currentDateMode === "range") {
          if (inputStart) inputStart.value = yestStr;
          if (inputEnd) inputEnd.value = yestStr;
          state.filterStartDate = yestStr;
          state.filterEndDate = yestStr;
        }
        loadDocuments();
      } else if (preset === "this-week") {
        const day = today.getDay(); // 0=Sun, ..., 6=Sat
        const diffToSat = (day + 1) % 7;
        const startOfWeek = new Date(today);
        startOfWeek.setDate(today.getDate() - diffToSat);
        const endOfWeek = new Date(startOfWeek);
        endOfWeek.setDate(startOfWeek.getDate() + 6);

        const startStr = formatDateIso(startOfWeek);
        const endStr = formatDateIso(endOfWeek);
        if (inputStart) inputStart.value = startStr;
        if (inputEnd) inputEnd.value = endStr;
        state.filterStartDate = startStr;
        state.filterEndDate = endStr;
        loadDocuments();
      } else if (preset === "this-month") {
        const firstDay = new Date(today.getFullYear(), today.getMonth(), 1);
        const lastDay = new Date(today.getFullYear(), today.getMonth() + 1, 0);
        const startStr = formatDateIso(firstDay);
        const endStr = formatDateIso(lastDay);
        if (inputStart) inputStart.value = startStr;
        if (inputEnd) inputEnd.value = endStr;
        state.filterStartDate = startStr;
        state.filterEndDate = endStr;
        loadDocuments();
      }
    });
  });
}

export function resetDateFilterUI() {
  document.querySelectorAll(".date-tab-btn").forEach((btn) => {
    const active = btn.getAttribute("data-mode") === "all";
    btn.classList.toggle("active", active);
    btn.setAttribute("aria-selected", active ? "true" : "false");
  });
  const panel = document.getElementById("date-filter-panel");
  if (panel) panel.style.display = "none";
  const singleInput = document.getElementById("filter-single-date");
  if (singleInput) singleInput.value = "";
  const startInput = document.getElementById("filter-start-date");
  if (startInput) startInput.value = "";
  const endInput = document.getElementById("filter-end-date");
  if (endInput) endInput.value = "";
  const dateFieldSelect = document.getElementById("filter-date-field");
  if (dateFieldSelect) dateFieldSelect.value = "doc_date";
  const rangeError = document.getElementById("date-range-error");
  if (rangeError) rangeError.style.display = "none";
}
