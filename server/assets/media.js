// Enhance server-rendered media and newly appended pages. Native image links
// and the original fenced code remain usable without this module.
const state = JSON.parse(document.querySelector("#state").textContent);
const t = (key) => state.messages[key];
const selector = ".prose pre > code.language-mermaid";
let renderer;
function openRenderer() {
  if (renderer) return renderer;
  const frame = document.createElement("iframe");
  frame.className = "diagram-worker";
  frame.tabIndex = -1;
  frame.setAttribute("aria-hidden", "true");
  frame.title = "Mermaid";
  frame.sandbox = "allow-scripts";
  frame.src = "/assets/diagram.html";
  renderer = new Promise((resolve, reject) => {
    const timeout = setTimeout(fail, 20000);
    function fail() {
      clearTimeout(timeout);
      frame.remove();
      renderer = undefined;
      reject(Error("renderer unavailable"));
    }
    frame.onload = () => {
      clearTimeout(timeout);
      resolve(frame);
    };
    frame.onerror = fail;
    document.body.append(frame);
  });
  return renderer;
}

async function draw(code) {
  if (!code.isConnected) return;
  const pre = code.parentNode;
  try {
    const frame = await openRenderer();
    const result = await new Promise((resolve, reject) => {
      const { port1, port2 } = new MessageChannel();
      const timer = setTimeout(() => {
        port1.close();
        reject(Error());
      }, 15000);
      port1.onmessage = ({ data }) => {
        clearTimeout(timer);
        port1.close();
        resolve(data);
      };
      frame.contentWindow.postMessage(code.textContent, "*", [port2]);
    });
    if (!result?.svg) throw Error();
    const image = new Image();
    image.alt = [result.title || t("diagram_title"), result.description]
      .filter(Boolean)
      .join(" — ");
    image.src =
      "data:image/svg+xml;charset=utf-8," + encodeURIComponent(result.svg);
    await image.decode();
    if (!pre.isConnected) return;
    const figure = document.createElement("figure");
    figure.className = "mermaid-figure";
    const trigger = imageButton(image);
    const details = document.createElement("details");
    const summary = document.createElement("summary");
    summary.textContent = t("diagram_source");
    details.append(summary);
    pre.replaceWith(figure);
    details.append(pre);
    figure.append(trigger, details);
  } catch {
    if (!pre.isConnected) return;
    const message = document.createElement("p");
    message.className = "diagram-error";
    message.textContent = t("diagram_error");
    pre.before(message);
  }
}

let queue = Promise.resolve();
function enqueue(code) {
  queue = queue.then(() => draw(code));
}
const observer =
  "IntersectionObserver" in window &&
  new IntersectionObserver(
    (entries) => {
      for (const entry of entries)
        if (entry.isIntersecting) {
          observer.unobserve(entry.target);
          enqueue(entry.target);
        }
    },
    { rootMargin: "200px" },
  );

function imageButton(img) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "media-trigger";
  button.setAttribute("aria-haspopup", "dialog");
  button.setAttribute(
    "aria-label",
    t("viewer_open").replace("{name}", img.alt || t("viewer_image")),
  );
  button.append(img);
  return button;
}

function enhance(root) {
  const select = (query) =>
    root.matches?.(query) ? [root] : root.querySelectorAll(query);
  for (const code of select(selector)) {
    if (code.dataset.diagram) continue;
    code.dataset.diagram = "pending";
    if (observer) observer.observe(code);
    else enqueue(code);
  }
  for (const img of select(
    ".prose img, .comment-text img, .attachment, .comment-images img",
  )) {
    if (img.closest(".media-trigger, .mermaid-figure") || img.dataset.viewer)
      continue;
    img.dataset.viewer = "ready";
    const link = img.closest("a");
    if (link) {
      // Preserve an author's link when a Markdown illustration points elsewhere.
      if (
        link.href !== img.src &&
        !img.matches(".attachment, .comment-images img")
      ) {
        const open = document.createElement("button");
        open.type = "button";
        open.className = "linked-image-view";
        open.textContent = t("viewer_open").replace(
          "{name}",
          img.alt || t("viewer_image"),
        );
        open.setAttribute("aria-haspopup", "dialog");
        open.addEventListener("click", () => show(open, img, img.src));
        link.after(open);
      } else {
        link.dataset.viewMedia = "";
        link.setAttribute("aria-haspopup", "dialog");
      }
    } else {
      const position = img.nextSibling,
        parent = img.parentNode;
      parent.insertBefore(imageButton(img), position);
    }
  }
}

async function show(trigger, img, src) {
  try {
    const { openViewer } = await import(state.viewerModule);
    openViewer(trigger, img.alt || t("viewer_image"), src, t);
  } catch {
    if (trigger.href) {
      location.assign(trigger.href);
      return;
    }
    const error = document.createElement("p");
    error.role = "alert";
    error.textContent = t("viewer_error");
    if (trigger.nextElementSibling?.role !== "alert") trigger.after(error);
  }
}

document.addEventListener("click", (event) => {
  if (
    event.button ||
    event.ctrlKey ||
    event.metaKey ||
    event.altKey ||
    event.shiftKey
  )
    return;
  const trigger = event
    .composedPath()
    .find((el) =>
      el.matches?.(".media-trigger, a[data-view-media], .comment-images a"),
    );
  if (!trigger) return;
  const img = trigger.querySelector("img");
  if (!img) return;
  event.preventDefault();
  show(trigger, img, trigger.href || img.currentSrc || img.src);
});
enhance(document);
new MutationObserver((records) => {
  for (const { addedNodes } of records)
    for (const node of addedNodes) {
      if (node.nodeType === 1 && node.closest(".themed")) enhance(node);
    }
}).observe(document.querySelector(".themed"), {
  childList: true,
  subtree: true,
});
