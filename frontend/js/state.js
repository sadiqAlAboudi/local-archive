// Application State Management

export const state = {
  currentTypeFilter: "all",
  searchQuery: "",
  currentPreviewDoc: null,
  currentDateMode: "all", // "all" | "single" | "range"
  filterStartDate: "",
  filterEndDate: "",
  filterDateField: "doc_date",
};

export function isDateFilterActive() {
  if (state.currentDateMode === "single") {
    return Boolean(state.filterStartDate);
  }
  if (state.currentDateMode === "range") {
    return Boolean(state.filterStartDate || state.filterEndDate);
  }
  return false;
}

export function hasActiveFilter() {
  return (
    state.searchQuery.trim() !== "" ||
    state.currentTypeFilter !== "all" ||
    isDateFilterActive()
  );
}

export function resetFilterState() {
  state.currentTypeFilter = "all";
  state.searchQuery = "";
  state.currentDateMode = "all";
  state.filterStartDate = "";
  state.filterEndDate = "";
  state.filterDateField = "doc_date";
}
