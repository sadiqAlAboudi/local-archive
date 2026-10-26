// Utility Functions

// Escape HTML characters to prevent XSS
export function escapeHtml(str) {
  if (!str) return "";
  return String(str)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

// Show alert banner notification
export function showAlert(message, type = "info") {
  const container = document.getElementById("alert-container");
  if (!container) return;

  const aside = document.createElement("aside");
  aside.setAttribute("role", type === "error" ? "alert" : "status");
  aside.innerHTML = `
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      ${
        type === "error"
          ? '<circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line>'
          : '<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline>'
      }
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

// Format Date object to local YYYY-MM-DD (avoiding timezone offset shifts)
export function formatDateIso(d) {
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}
