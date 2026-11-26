// Public reading works without JavaScript. Feature-detect browser agent tools.
const readerState = JSON.parse(document.querySelector("#state").textContent);
(() => {
  const api = document.modelContext;
  if (!api?.registerTool) return;
  for (const tool of readerState.tools) {
    api.registerTool({
      name: tool.name,
      description: readerState.messages[tool.name],
      inputSchema: tool.inputSchema,
      async execute(input) {
        const response = await fetch("/api/v1/" + tool.name, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Th0ughts-Request": "1",
          },
          body: JSON.stringify(input),
        });
        const result = await response.json();
        return {
          content: [{ type: "text", text: JSON.stringify(result) }],
          isError: !response.ok,
        };
      },
    });
  }
})();

const readerLocale = document.documentElement.lang;
const dateFormat = new Intl.DateTimeFormat(readerLocale, {
  dateStyle: "long",
  timeStyle: "short",
});
const relativeFormat = new Intl.RelativeTimeFormat(readerLocale, { numeric: "auto" });

function formatDates(root) {
  for (const el of root.querySelectorAll("time[datetime]")) {
    const date = new Date(el.dateTime);
    if (!Number.isFinite(date.getTime())) continue;
    el.title = dateFormat.format(date);
    const minutes = Math.floor((Date.now() - date.getTime()) / 60000);
    if (minutes >= 0 && minutes < 1440) {
      el.textContent = relativeFormat.format(
        minutes < 60 ? -minutes : -Math.floor(minutes / 60),
        minutes < 60 ? "minute" : "hour",
      );
    }
  }
}
formatDates(document);

const picker = document.querySelector(".language-picker");
if (picker) {
  document.addEventListener("click", (event) => {
    if (picker.open && !picker.contains(event.target)) picker.open = false;
  });
  picker.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && picker.open) {
      picker.open = false;
      picker.querySelector("summary").focus();
    }
  });
}

// The regular cursor link remains navigable without browser enhancement.
function enableFeedScroll(list, link) {
  const status = link.nextElementSibling;
  const label = link.textContent;
  const seen = new Set(Array.from(list.children, (item) => item.dataset.postId));
  const mediaType = "application/vnd.th0ughts.timeline+json";
  let loading = false;
  const observer = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting && entry.target === list.lastElementChild)) load();
  }, { rootMargin: "0px 0px 400px" });

  async function load() {
    if (loading || !link.isConnected) return;
    loading = true;
    observer.disconnect();
    list.setAttribute("aria-busy", "true");
    link.textContent = link.dataset.loading;
    status.textContent = link.dataset.loading;
    try {
      const response = await fetch(link.href, {
        headers: { Accept: mediaType },
        mode: "same-origin",
        redirect: "error",
      });
      if (!response.ok) throw new Error();
      const page = await response.json();
      if (page.owner !== readerState.owner || page.lang !== readerLocale || page.theme !== list.dataset.feedTheme) {
        // A session, language or theme change requires a fresh document.
        link.removeEventListener("click", follow);
        link.textContent = label;
        status.textContent = "";
        return;
      }
      const template = document.createElement("template");
      template.innerHTML = page.items;
      let first;
      let count = 0;
      for (const item of Array.from(template.content.children)) {
        if (seen.has(item.dataset.postId)) {
          item.remove();
          continue;
        }
        seen.add(item.dataset.postId);
        item.dataset.scrollItem = "";
        first ??= item;
        count++;
      }
      formatDates(template.content);
      list.append(template.content);
      if (first && document.activeElement === link) {
        first.tabIndex = -1;
        first.focus();
      }
      status.textContent = link.dataset.loaded.replace("{n}", count);
      if (!page.next) {
        link.remove();
        return;
      }
      link.href = page.next;
      link.textContent = label;
      observer.observe(list.lastElementChild);
    } catch {
      // Keep the cursor for an explicit retry. Do not loop on service failures.
      link.textContent = link.dataset.retry;
      status.textContent = link.dataset.retry;
    } finally {
      list.removeAttribute("aria-busy");
      loading = false;
    }
  }

  function follow(event) {
    if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    load();
  }
  link.addEventListener("click", follow);
  observer.observe(list.lastElementChild);
}

const timeline = document.querySelector("[data-feed-theme]");
const nextPage = document.querySelector("a[data-feed-next]");
if (nextPage && timeline?.lastElementChild && "IntersectionObserver" in window) {
  enableFeedScroll(timeline, nextPage);
}
