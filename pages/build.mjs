// Build a static project site. Both languages render before React hydrates.
import { cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { execFileSync } from "node:child_process";
import React from "react";
import { renderToString } from "react-dom/server";
import { build } from "esbuild";
import { App } from "./App.jsx";
import { site, text } from "./content.js";

const root = fileURLToPath(new URL("../", import.meta.url));
const dist = path.join(root, "pages/dist");
const ref = execFileSync("git", ["rev-parse", "HEAD"], {
  cwd: root,
  encoding: "utf8",
}).trim();
if (!/^[0-9a-f]{40}$/.test(ref))
  throw Error("A Git source revision is required");
await rm(dist, { recursive: true, force: true });
await mkdir(path.join(dist, "tr"), { recursive: true });
await cp(path.join(root, "pages/public"), dist, { recursive: true });
await build({
  absWorkingDir: root,
  entryPoints: { app: "pages/client.jsx", styles: "pages/styles.css" },
  outdir: dist,
  bundle: true,
  format: "esm",
  minify: true,
  define: { "process.env.NODE_ENV": '"production"' },
  loader: { ".woff2": "file", ".woff": "file", ".jpg": "file" },
  assetNames: "assets/[name]-[hash]",
});
for (const name of ["instrument-serif", "dm-sans"]) {
  const pkg =
    name === "dm-sans"
      ? "@fontsource-variable/dm-sans"
      : "@fontsource/instrument-serif";
  await cp(
    path.join(root, "node_modules", pkg, "LICENSE"),
    path.join(dist, "assets", name + "-LICENSE.txt"),
  );
}
for (const lang of ["en", "tr"]) {
  const tr = lang === "tr",
    prefix = tr ? "../" : "./",
    url = site + (tr ? "tr/" : "");
  const title = tr
    ? "th0ughts · Kendi sunucunda mikroblog"
    : "th0ughts · A self-hosted microblog";
  const description = text[lang].lead;
  const markup = renderToString(React.createElement(App, { lang }));
  const html = `<!doctype html><html lang="${lang}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${title}</title><meta name="description" content="${description}"><link rel="canonical" href="${url}"><link rel="alternate" hreflang="en" href="${site}"><link rel="alternate" hreflang="tr" href="${site}tr/"><link rel="alternate" hreflang="x-default" href="${site}"><meta property="og:type" content="website"><meta property="og:url" content="${url}"><meta property="og:title" content="${title}"><meta property="og:description" content="${description}"><meta property="og:image" content="${site}screenshots/timeline.webp"><meta name="twitter:card" content="summary_large_image"><meta name="theme-color" content="#152b3c"><link rel="stylesheet" href="${prefix}styles.css"></head><body><div id="root">${markup}</div><script type="module" src="${prefix}app.js"></script></body></html>\n`;
  await writeFile(path.join(dist, tr ? "tr/index.html" : "index.html"), html);
}
const installer = await readFile(path.join(root, "deploy/install.sh"), "utf8");
if (!installer.includes("\nSOURCE_REF=main\n"))
  throw Error("Missing installer revision marker");
await writeFile(
  path.join(dist, "install.sh"),
  installer.replace("\nSOURCE_REF=main\n", "\nSOURCE_REF=" + ref + "\n"),
);
await writeFile(path.join(dist, ".nojekyll"), "");
