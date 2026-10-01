document.addEventListener("DOMContentLoaded", function () {
  const toast = document.querySelector("#admin-toast");
  let toastTimer;
  const showToast = function (message) {
    if (!toast || !message) return;
    toast.textContent = message;
    toast.classList.add("is-visible");
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(function () {
      toast.classList.remove("is-visible");
    }, 3200);
  };

  if (toast && toast.textContent.trim()) {
    showToast(toast.textContent.trim());
  }
  document.addEventListener("adminToast", function (event) {
    showToast(event.detail.message);
  });

  const dialog = document.querySelector(".admin-confirm");

  let pendingForm;
  document.addEventListener("submit", function (event) {
    const form = event.target.closest("form");
    if (!form) return;
    const confirmation = form.querySelector("[data-confirm]");
    if (!dialog || !confirmation || form.dataset.confirmed === "true") return;
    event.preventDefault();
    pendingForm = form;
    dialog.querySelector("#admin-confirm-message").textContent = confirmation.dataset.confirm;
    dialog.returnValue = "cancel";
    dialog.showModal();
  });
  if (dialog) {
    dialog.addEventListener("click", function (event) {
      if (event.target === dialog) {
        dialog.returnValue = "cancel";
        dialog.close();
      }
    });
    dialog.addEventListener("keydown", function (event) {
      if (event.key !== "Tab") return;
      const focusable = Array.from(dialog.querySelectorAll('button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'));
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    });
  }
  if (dialog) {
    dialog.addEventListener("close", function () {
      if (!pendingForm) return;
      const form = pendingForm;
      pendingForm = null;
      if (dialog.returnValue !== "confirm") return;
      form.dataset.confirmed = "true";
      form.requestSubmit();
    });
  }

  const title = document.querySelector("#article-title");
  const slug = document.querySelector("#article-slug");
  if (!title || !slug) return;

  let slugWasEdited = slug.value.length > 0;
  slug.addEventListener("input", function () {
    slugWasEdited = true;
  });
  title.addEventListener("input", function () {
    if (slugWasEdited) return;
    slug.value = title.value
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "");
  });
});
