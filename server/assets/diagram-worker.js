// This document has an opaque origin. Only exchange render results with its parent.
mermaid.initialize({
  startOnLoad: false,
  securityLevel: "strict",
  htmlLabels: false,
  maxTextSize: 32000,
  maxEdges: 500,
  suppressErrorRendering: true,
});
window.addEventListener("message", async (event) => {
  const port = event.ports[0];
  if (window.parent === window || event.source !== window.parent || !port)
    return;
  try {
    const source = event.data;
    if (typeof source !== "string" || source.length > 32000) throw Error();
    const { svg } = await mermaid.render("thought-diagram", source);
    const doc = new DOMParser().parseFromString(svg, "image/svg+xml");
    const root = doc.documentElement;
    if (root.localName !== "svg") throw Error();
    const bounds = root
      .getAttribute("viewBox")
      ?.trim()
      .split(/[ ,]+/)
      .map(Number);
    if (bounds?.length === 4 && bounds[2] > 0 && bounds[3] > 0) {
      root.setAttribute("width", bounds[2]);
      root.setAttribute("height", bounds[3]);
      root.style.removeProperty("max-width");
    }
    port.postMessage({
      svg: new XMLSerializer().serializeToString(root),
      title: root.querySelector("title")?.textContent,
      description: root.querySelector("desc")?.textContent,
    });
  } catch {
    port.postMessage(null);
  } finally {
    port.close();
  }
});
