// Scanner Studio Integration (WIA Scanner, Multi-Page Preview, Rotation, Feeder ADF Batch, PDF Compilation)
import { showAlert } from "./utils.js";

let scannedPages = [];
let isScanning = false;
let onCompleteCallback = null;

// Initialize Scanner Studio UI elements, mode toggles, and event handlers
export function initScannerStudio() {
  const scannerDialog = document.getElementById("scanner-dialog");
  if (!scannerDialog) return;

  const btnScanNew = document.getElementById("btn-scan-new");
  const btnCompile = document.getElementById("btn-compile-pdf");
  const btnCancel = document.getElementById("btn-scanner-cancel");
  const btnClose = scannerDialog.querySelector(".scanner-close-btn");

  // Mode Selector (Flatbed vs Feeder/ADF)
  const modeRadios = scannerDialog.querySelectorAll('input[name="scan_source_mode"]');
  const duplexToggle = document.getElementById("scanner-duplex-toggle");
  const btnScanLabel = document.getElementById("btn-scan-label");

  const updateModeState = () => {
    const selectedMode = scannerDialog.querySelector('input[name="scan_source_mode"]:checked')?.value || "flatbed";
    const isFeeder = selectedMode === "feeder";

    scannerDialog.querySelectorAll(".scanner-mode-pill").forEach((pill) => {
      const radio = pill.querySelector('input[type="radio"]');
      pill.classList.toggle("active", radio && radio.checked);
    });

    if (duplexToggle) {
      duplexToggle.style.display = isFeeder ? "inline-flex" : "none";
    }

    if (btnScanLabel) {
      btnScanLabel.textContent = isFeeder ? "بدء مسح حزمة الأوراق (ADF)" : "مسح صفحة جديدة";
    }
  };

  modeRadios.forEach((radio) => {
    radio.addEventListener("change", updateModeState);
  });
  updateModeState();

  // Listen to real-time streamed page events from Go backend during ADF feeder scan
  if (window.runtime && typeof window.runtime.EventsOn === "function") {
    window.runtime.EventsOn("scanner:page_scanned", (page) => {
      if (page && page.temp_path) {
        // Prevent duplicate appending if already recorded
        const exists = scannedPages.some((p) => p.temp_path === page.temp_path);
        if (!exists) {
          scannedPages.push({
            id: page.id || `page_${Date.now()}`,
            temp_path: page.temp_path,
            base64: page.base64,
            width: page.width,
            height: page.height,
            rotation: 0,
          });
          updateScannerUI();
        }
      }
    });
  }

  if (btnScanNew) {
    btnScanNew.addEventListener("click", () => {
      const selectedMode = scannerDialog.querySelector('input[name="scan_source_mode"]:checked')?.value || "flatbed";
      if (selectedMode === "feeder") {
        handleScanFeeder();
      } else {
        handleScanPage();
      }
    });
  }

  if (btnCompile) {
    btnCompile.addEventListener("click", () => handleCompilePDF());
  }

  const cancelHandler = async () => {
    await discardScanSession();
    scannerDialog.close();
  };

  if (btnCancel) btnCancel.addEventListener("click", cancelHandler);
  if (btnClose) btnClose.addEventListener("click", cancelHandler);

  // Light dismiss on backdrop click
  scannerDialog.addEventListener("click", async (e) => {
    const rect = scannerDialog.getBoundingClientRect();
    const inDialog =
      rect.top <= e.clientY &&
      e.clientY <= rect.top + rect.height &&
      rect.left <= e.clientX &&
      e.clientX <= rect.left + rect.width;
    if (!inDialog) {
      await discardScanSession();
      scannerDialog.close();
    }
  });
}

// Open Scanner Studio modal with a callback when PDF is compiled
export function openScannerStudio(callback) {
  onCompleteCallback = callback;
  scannedPages = [];
  updateScannerUI();

  const scannerDialog = document.getElementById("scanner-dialog");
  if (scannerDialog) {
    scannerDialog.showModal();
  }
}

// Discard all current scanned temp files
export async function discardScanSession() {
  if (scannedPages.length > 0) {
    const paths = scannedPages.map((p) => p.temp_path).filter(Boolean);
    scannedPages = [];
    updateScannerUI();
    try {
      if (paths.length > 0 && window.go && window.go.main && window.go.main.App) {
        await window.go.main.App.CleanupTempScans(paths);
      }
    } catch (err) {
      console.warn("CleanupTempScans warning:", err);
    }
  }
}

// Check if there are active pages
export function hasActiveScanPages() {
  return scannedPages.length > 0;
}

// Handle scanning a single page (Flatbed mode)
async function handleScanPage() {
  if (isScanning) return;
  isScanning = true;
  setScanningStatus(true, "جاري الاتصال بالماسح الضوئي وسحب الصفحة... يرجى الانتظار");

  try {
    const page = await window.go.main.App.ScanPage();
    if (page && page.temp_path) {
      scannedPages.push({
        id: page.id || `page_${Date.now()}`,
        temp_path: page.temp_path,
        base64: page.base64,
        width: page.width,
        height: page.height,
        rotation: 0,
      });
      updateScannerUI();
    }
  } catch (err) {
    const errStr = String(err || "");
    if (errStr.includes("scan_canceled") || errStr.includes("canceled") || errStr.includes("cancelled")) {
      // User canceled the native Windows scan dialog - no alarm needed
    } else {
      showAlert(errStr || "حدث خطأ أثناء محاولة المسح الضوئي", "error");
    }
  } finally {
    isScanning = false;
    setScanningStatus(false);
  }
}

// Handle scanning multiple pages from Automatic Document Feeder (ADF Batch mode)
async function handleScanFeeder() {
  if (isScanning) return;
  isScanning = true;
  setScanningStatus(true, "جاري سحب الأوراق عبر وحدة التغذية (ADF)... يرجى الانتظار");

  const duplex = document.getElementById("scan-duplex-checkbox")?.checked || false;
  const initialCount = scannedPages.length;

  try {
    const batchPages = await window.go.main.App.ScanFeeder(duplex, 100);
    if (batchPages && batchPages.length > 0) {
      // Merge any pages that weren't caught by the real-time event
      batchPages.forEach((p) => {
        if (!scannedPages.some((sp) => sp.temp_path === p.temp_path)) {
          scannedPages.push(p);
        }
      });
      updateScannerUI();
      const addedCount = scannedPages.length - initialCount;
      showAlert(`تم مسح ${addedCount} صفحة عبر وحدة التغذية (ADF) بنجاح`, "success");
    }
  } catch (err) {
    const errStr = String(err || "");
    if (errStr.includes("scan_canceled") || errStr.includes("canceled") || errStr.includes("cancelled")) {
      // Intentional cancellation
    } else {
      // If some pages were scanned before a jam or error, keep them
      updateScannerUI();
      showAlert(errStr || "حدث خطأ أثناء سحب الأوراق من وحدة التغذية", "error");
    }
  } finally {
    isScanning = false;
    setScanningStatus(false);
  }
}

// Rotate page 90 degrees clockwise
async function handleRotatePage(index) {
  const page = scannedPages[index];
  if (!page || !page.temp_path) return;

  const card = document.querySelector(`[data-scan-idx="${index}"]`);
  const imgEl = card ? card.querySelector(".scanner-thumb-img") : null;
  const rotateBtn = card ? card.querySelector(".btn-card-rotate") : null;

  if (rotateBtn) rotateBtn.disabled = true;
  if (imgEl) imgEl.style.opacity = "0.5";

  try {
    const updated = await window.go.main.App.RotatePage(page.temp_path, 90);
    if (updated) {
      page.temp_path = updated.temp_path;
      page.base64 = updated.base64;
      page.width = updated.width;
      page.height = updated.height;
      if (imgEl) {
        imgEl.src = updated.base64;
      }
    }
  } catch (err) {
    showAlert("تعذر تدوير الصفحة: " + err, "error");
  } finally {
    if (rotateBtn) rotateBtn.disabled = false;
    if (imgEl) imgEl.style.opacity = "1";
  }
}

// Delete page and remove its temp file
async function handleDeletePage(index) {
  const page = scannedPages[index];
  if (!page) return;

  const tempPath = page.temp_path;
  scannedPages.splice(index, 1);
  updateScannerUI();

  if (tempPath && window.go && window.go.main && window.go.main.App) {
    try {
      await window.go.main.App.DeleteTempScan(tempPath);
    } catch (err) {
      console.warn("DeleteTempScan warning:", err);
    }
  }
}

// Compile all pages into a standard A4 PDF
async function handleCompilePDF() {
  if (scannedPages.length === 0) {
    showAlert("يرجى مسح صفحة واحدة على الأقل قبل التجميع", "warning");
    return;
  }

  const btnCompile = document.getElementById("btn-compile-pdf");
  if (btnCompile) btnCompile.disabled = true;
  setScanningStatus(true, "جاري تجميع الصفحات في ملف PDF (A4)...");

  try {
    const paths = scannedPages.map((p) => p.temp_path);
    const pdfPath = await window.go.main.App.SaveToPDF(paths, "مستند_ممسوح.pdf");

    if (pdfPath) {
      const pageCount = scannedPages.length;
      scannedPages = []; // Temp image files are already cleaned up by backend SaveToPDF
      updateScannerUI();

      const scannerDialog = document.getElementById("scanner-dialog");
      if (scannerDialog) scannerDialog.close();

      if (typeof onCompleteCallback === "function") {
        onCompleteCallback(pdfPath, pageCount);
      }
      showAlert(`تم تجميع ${pageCount} صفحة في مستند PDF بنجاح`, "success");
    }
  } catch (err) {
    showAlert("فشل تجميع ملف PDF: " + err, "error");
  } finally {
    if (btnCompile) btnCompile.disabled = false;
    setScanningStatus(false);
  }
}

// Set scanning status and spinner
function setScanningStatus(active, text = "") {
  const statusEl = document.getElementById("scanner-status-indicator");
  const statusText = document.getElementById("scanner-status-text");
  const btnScanNew = document.getElementById("btn-scan-new");

  if (statusEl) {
    statusEl.classList.toggle("active", active);
  }
  if (statusText) {
    statusText.textContent = text;
  }
  if (btnScanNew) {
    btnScanNew.disabled = active;
  }
}

// Re-render pages grid, counter, and empty state
function updateScannerUI() {
  const grid = document.getElementById("scanner-grid");
  const emptyState = document.getElementById("scanner-empty-state");
  const countDisplay = document.getElementById("scanner-count-display");
  const btnCompile = document.getElementById("btn-compile-pdf");

  const count = scannedPages.length;

  if (countDisplay) {
    countDisplay.innerHTML = `الصفحات الممسوحة: <strong>${count}</strong>`;
  }

  if (btnCompile) {
    btnCompile.disabled = count === 0;
  }

  if (!grid || !emptyState) return;

  if (count === 0) {
    grid.style.display = "none";
    grid.innerHTML = "";
    emptyState.style.display = "flex";
    return;
  }

  emptyState.style.display = "none";
  grid.style.display = "grid";
  grid.innerHTML = "";

  scannedPages.forEach((page, index) => {
    const card = document.createElement("article");
    card.className = "scanner-card";
    card.setAttribute("data-scan-idx", index);

    card.innerHTML = `
      <header class="scanner-card-header">
        <span>صفحة ${index + 1}</span>
        <span class="scanner-card-badge">${page.width} &times; ${page.height}</span>
      </header>
      <div class="scanner-thumb-container">
        <img class="scanner-thumb-img" src="${page.base64}" alt="معاينة صفحة ${index + 1}" loading="lazy">
      </div>
      <footer class="scanner-card-actions">
        <button type="button" class="btn-card-action btn-card-rotate" title="تدوير 90 درجة باتجاه عقارب الساعة">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="23 4 23 10 17 10"></polyline>
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path>
          </svg>
          تدوير 90°
        </button>
        <button type="button" class="btn-card-action btn-card-delete" title="حذف هذه الصفحة">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="3 6 5 6 21 6"></polyline>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
          </svg>
          حذف
        </button>
      </footer>
    `;

    const rotateBtn = card.querySelector(".btn-card-rotate");
    if (rotateBtn) {
      rotateBtn.addEventListener("click", () => handleRotatePage(index));
    }

    const deleteBtn = card.querySelector(".btn-card-delete");
    if (deleteBtn) {
      deleteBtn.addEventListener("click", () => handleDeletePage(index));
    }

    grid.appendChild(card);
  });
}
