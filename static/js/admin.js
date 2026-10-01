document.addEventListener("DOMContentLoaded", function () {
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
