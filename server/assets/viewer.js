// Transform around the pointer in stage coordinates, with limits based on the
// fitted image size. Pointer Events cover mouse, pen, drag and two-finger pinch.
export function openViewer(trigger, alt, src, t) {
  if (document.querySelector(".media-viewer[open]")) return;
  const dialog = document.createElement("dialog");
  dialog.className = "media-viewer";
  dialog.setAttribute("aria-labelledby", "viewer-title");
  const heading = document.createElement("h2");
  heading.id = "viewer-title";
  heading.textContent = alt;
  const bar = document.createElement("div");
  bar.className = "viewer-bar";
  const stage = document.createElement("div");
  stage.className = "viewer-stage";
  stage.tabIndex = 0;
  stage.role = "group";
  stage.setAttribute("aria-label", t("viewer_stage"));
  stage.setAttribute("aria-describedby", "viewer-help");
  const help = document.createElement("p");
  help.id = "viewer-help";
  help.textContent = t("viewer_help");
  const controls = document.createElement("div");
  controls.className = "viewer-controls";
  const zoomControls = document.createElement("div");
  const panControls = document.createElement("div");
  zoomControls.className = panControls.className = "viewer-control-group";
  controls.append(zoomControls, panControls);
  const status = document.createElement("output");
  status.setAttribute("aria-label", t("viewer_zoom"));
  const img = new Image();
  img.alt = alt;
  img.draggable = false;
  const diagram = !!trigger.closest(".mermaid-figure");
  if (diagram) img.className = "diagram-image";
  stage.append(img);

  let scale = 1,
    x = 0,
    y = 0,
    locked = false;
  const points = new Map();
  const bound = (n, min, max) => Math.min(max, Math.max(n, min));
  function button(parent, key, text, action) {
    const b = document.createElement("button");
    b.type = "button";
    b.title = t(key);
    b.setAttribute("aria-label", t(key));
    b.textContent = text;
    b.onclick = action;
    parent.append(b);
    return b;
  }
  function paint() {
    const fit = Math.min(
      stage.clientWidth / (img.naturalWidth || 1),
      stage.clientHeight / (img.naturalHeight || 1),
      diagram ? Infinity : 1,
    );
    const width = img.naturalWidth * fit,
      height = img.naturalHeight * fit;
    const maxX = Math.max(0, (width * scale - stage.clientWidth) / 2);
    const maxY = Math.max(0, (height * scale - stage.clientHeight) / 2);
    x = bound(x, -maxX, maxX);
    y = bound(y, -maxY, maxY);
    img.style.width = width + "px";
    img.style.height = height + "px";
    img.style.transform = `translate(-50%, -50%) translate(${x}px, ${y}px) scale(${scale})`;
    const percent = Math.round(scale * 100) + "%";
    if (status.textContent !== percent) status.textContent = percent;
    decrease.disabled = locked || scale <= 1;
    increase.disabled = locked || scale >= 12;
    reset.disabled = locked || scale === 1;
    for (const [b, available] of [
      [left, x < maxX],
      [right, x > -maxX],
      [up, y < maxY],
      [down, y > -maxY],
    ])
      b.disabled = locked || !available;
  }
  function zoom(next, at = { x: 0, y: 0 }, dest = at) {
    if (locked) return;
    const factor = bound(next, 1, 12) / scale;
    x = dest.x - (at.x - x) * factor;
    y = dest.y - (at.y - y) * factor;
    scale = bound(next, 1, 12);
    paint();
  }
  function pan(dx, dy) {
    if (!locked) {
      x += dx;
      y += dy;
      paint();
    }
  }
  bar.append(heading);
  const close = button(bar, "close", "×", () => dialog.close());
  const decrease = button(zoomControls, "viewer_out", "−", () =>
    zoom(scale / 1.5),
  );
  zoomControls.append(status);
  const increase = button(zoomControls, "viewer_in", "+", () =>
    zoom(scale * 1.5),
  );
  const reset = button(zoomControls, "viewer_fit", t("viewer_fit"), () =>
    zoom(1),
  );
  const left = button(panControls, "viewer_left", "←", () => pan(72, 0));
  const right = button(panControls, "viewer_right", "→", () => pan(-72, 0));
  const up = button(panControls, "viewer_up", "↑", () => pan(0, 72));
  const down = button(panControls, "viewer_down", "↓", () => pan(0, -72));
  const pin = button(controls, "viewer_pin", t("viewer_pin"), () => {
    locked = !locked;
    points.clear();
    pin.setAttribute("aria-pressed", String(locked));
    paint();
  });
  pin.setAttribute("aria-pressed", "false");
  function point(e) {
    const box = stage.getBoundingClientRect();
    return {
      x: e.clientX - box.left - box.width / 2,
      y: e.clientY - box.top - box.height / 2,
    };
  }
  function gesture() {
    const [a, b] = points.values();
    return b
      ? {
          x: (a.x + b.x) / 2,
          y: (a.y + b.y) / 2,
          d: Math.hypot(a.x - b.x, a.y - b.y),
        }
      : a;
  }
  // The stage handles direct manipulation itself. Suppress the browser's
  // compatibility taps after a drag or pinch while keeping native toolbar taps.
  stage.addEventListener("touchstart", (e) => e.preventDefault(), {
    passive: false,
  });
  stage.onpointerdown = (e) => {
    if (locked || e.button !== 0 || points.size >= 2) return;
    points.set(e.pointerId, point(e));
    stage.setPointerCapture(e.pointerId);
  };
  stage.onpointermove = (e) => {
    if (!points.has(e.pointerId)) return;
    const old = gesture();
    points.set(e.pointerId, point(e));
    const next = gesture();
    if (points.size === 2 && old.d > 0)
      zoom((scale * next.d) / old.d, old, next);
    else pan(next.x - old.x, next.y - old.y);
  };
  stage.onlostpointercapture =
    stage.onpointerup =
    stage.onpointercancel =
      (e) => points.delete(e.pointerId);
  stage.addEventListener(
    "wheel",
    (e) => {
      e.preventDefault();
      zoom(scale * Math.exp(-bound(e.deltaY, -200, 200) * 0.006), point(e));
    },
    { passive: false },
  );
  stage.ondblclick = (e) => zoom(scale === 1 ? 2 : 1, point(e));
  dialog.onkeydown = (e) => {
    if (e.ctrlKey || e.altKey || e.metaKey) return;
    const actions = {
      "+": increase,
      "=": increase,
      "-": decrease,
      0: reset,
      ArrowLeft: left,
      ArrowRight: right,
      ArrowUp: up,
      ArrowDown: down,
    };
    if (Object.hasOwn(actions, e.key)) {
      e.preventDefault();
      actions[e.key].click();
    }
  };
  dialog.addEventListener("click", (e) => {
    if (e.target === dialog && !bar.contains(e.target)) {
      const b = dialog.getBoundingClientRect();
      if (
        e.clientX < b.left ||
        e.clientX > b.right ||
        e.clientY < b.top ||
        e.clientY > b.bottom
      )
        dialog.close();
    }
  });
  let imageURL = src;
  if (trigger.dataset.still) {
    let playing = !matchMedia("(prefers-reduced-motion: reduce)").matches;
    imageURL = playing ? src : trigger.dataset.still;
    const animate = button(
      controls,
      "viewer_animation",
      t("viewer_animation"),
      () => {
        playing = !playing;
        img.src = playing ? src : trigger.dataset.still;
        animate.setAttribute("aria-pressed", String(playing));
      },
    );
    animate.setAttribute("aria-pressed", String(playing));
  }
  const resize = new ResizeObserver(paint);
  dialog.onclose = () => {
    resize.disconnect();
    dialog.remove();
    document.documentElement.classList.remove("viewer-open");
    if (trigger.isConnected) trigger.focus({ preventScroll: true });
  };
  dialog.append(bar, stage, controls, help);
  document.body.append(dialog);
  dialog.showModal();
  close.focus();
  document.documentElement.classList.add("viewer-open");
  resize.observe(stage);
  img.onload = paint;
  img.onerror = () => {
    help.textContent = t("viewer_error");
    help.role = "alert";
  };
  img.src = imageURL;
  paint();
}
