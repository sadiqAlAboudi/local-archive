import { refreshStats } from "./stats.js";
import { loadDocuments, setupSearchAndFilterListeners } from "./documents.js";
import { setupDateFilterListeners } from "./date-filter.js";
import { setupDialogs } from "./dialogs.js";
import { setupPreviewListeners } from "./preview.js";
import { setupBackupListeners } from "./backup.js";
import { setupUpdateListeners, checkForUpdates } from "./updater.js";
import { initScannerStudio } from "./scanner.js";

// Initialize application on DOM ready
document.addEventListener("DOMContentLoaded", () => {
  // Setup dialogs (open, close, light dismiss, form submissions)
  setupDialogs();

  // Setup scanner studio modal and events
  initScannerStudio();

  // Setup search input and doc-type filter tabs
  setupSearchAndFilterListeners();

  // Setup date filter toolbar and preset buttons
  setupDateFilterListeners();

  // Setup preview modal controls (print, iframe/image toggle)
  setupPreviewListeners();

  // Setup backup, restore, and database repair buttons
  setupBackupListeners();

  // Setup auto-update listeners and modal controls
  setupUpdateListeners();

  // Initial load of statistics and document records
  refreshStats();
  loadDocuments();

  // Check for updates silently on startup after short delay
  setTimeout(() => {
    checkForUpdates(true);
  }, 1500);
});
