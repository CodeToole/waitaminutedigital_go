/* Highlight carousel. Buttons are real <button>s, so Enter and Space work without this file. The script only moves the scrollport. */
(function () {
  const root = document.querySelector("[data-carousel]");
  if (!root) return;
  const track = root.querySelector("[data-carousel-track]");
  const prev = root.querySelector('[data-carousel="prev"]');
  const next = root.querySelector('[data-carousel="next"]');
  if (!track || !prev || !next) return;

  function step() {
    const card = track.querySelector(".highlight-card");
    if (!card) return track.clientWidth;
    const gap = parseFloat(getComputedStyle(track).columnGap || getComputedStyle(track).gap || "0") || 0;
    return card.getBoundingClientRect().width + gap;
  }

  function update() {
    const max = track.scrollWidth - track.clientWidth - 1;
    prev.disabled = track.scrollLeft <= 1;
    next.disabled = track.scrollLeft >= max;
  }

  function move(dir) {
    track.scrollBy({ left: dir * step(), behavior: "smooth" });
  }

  prev.addEventListener("click", function () { move(-1); });
  next.addEventListener("click", function () { move(1); });
  track.addEventListener("keydown", function (event) {
    if (event.key === "ArrowRight") { event.preventDefault(); move(1); }
    if (event.key === "ArrowLeft") { event.preventDefault(); move(-1); }
  });
  track.addEventListener("scroll", update, { passive: true });
  update();
})();

document.querySelectorAll("[data-copy-link]").forEach(function (button) {
  button.addEventListener("click", async function () {
    const pageURL = button.getAttribute("data-copy-link");
    const status = button.parentElement.querySelector("[data-copy-status]");
    let copied = false;

    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(pageURL);
        copied = true;
      } else {
        const field = document.createElement("textarea");
        field.value = pageURL;
        field.setAttribute("readonly", "");
        field.style.position = "fixed";
        field.style.opacity = "0";
        document.body.appendChild(field);
        field.select();
        copied = document.execCommand("copy");
        field.remove();
      }
    } catch {
      copied = false;
    }

    if (status) {
      status.textContent = copied ? "Copied!" : "Copy failed";
    }
  });
});
