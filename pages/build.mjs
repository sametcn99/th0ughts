// Build a static project site. Both languages render before React hydrates.
import { cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { execFileSync } from "node:child_process";
import React from "react";
import { renderToString } from "react-dom/server";
import { build } from "esbuild";
import { Documents } from "./Documents.jsx";
import { loadDocuments, renderDocument } from "./markdown.mjs";
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
await cp(
  path.join(root, "node_modules/@fontsource-variable/dm-sans/LICENSE"),
  path.join(dist, "assets/dm-sans-LICENSE.txt"),
);
const documents = await loadDocuments(root, dist);
const routes = new Map(documents.map((doc) => [doc.file, doc.route]));
const escape = (value) =>
  value
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
for (const lang of ["en", "tr"]) {
  const tr = lang === "tr";
  async function writePage(route, article) {
    const localRoute = (tr ? "tr/" : "") + route;
    const url = site + localRoute;
    const depth = localRoute.split("/").filter(Boolean).length;
    const prefix = "../".repeat(depth) || "./";
    const title = route
      ? (article?.title || text[lang].documents) + " · th0ughts"
      : tr
        ? "th0ughts · Kendi sunucunda mikroblog"
        : "th0ughts · A self-hosted microblog";
    const description = route ? title : text[lang].lead;
    const markup = renderToString(
      route
        ? React.createElement(Documents, {
            lang,
            prefix,
            route,
            documents,
            article,
            html: article && renderDocument(article, routes, prefix, lang),
          })
        : React.createElement(App, { lang }),
    );
    const script = route
      ? ""
      : `<script type="module" src="${prefix}app.js?v=${ref}"></script>`;
    const html = `<!doctype html><html lang="${lang}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${escape(title)}</title><meta name="description" content="${escape(description)}"><link rel="canonical" href="${url}"><link rel="alternate" hreflang="en" href="${site}${route}"><link rel="alternate" hreflang="tr" href="${site}tr/${route}"><link rel="alternate" hreflang="x-default" href="${site}${route}"><meta property="og:type" content="website"><meta property="og:url" content="${url}"><meta property="og:title" content="${escape(title)}"><meta property="og:description" content="${escape(description)}"><meta property="og:image" content="${site}screenshots/timeline.webp"><meta name="twitter:card" content="summary_large_image"><meta name="theme-color" content="#fafaf7"><link rel="stylesheet" href="${prefix}styles.css?v=${ref}"></head><body><div id="root">${markup}</div>${script}</body></html>\n`;
    const dir = path.join(dist, decodeURIComponent(localRoute));
    await mkdir(dir, { recursive: true });
    await writeFile(path.join(dir, "index.html"), html);
  }
  await writePage("");
  await writePage("docs/");
  for (const article of documents)
    await writePage("docs/" + article.route, article);
}
const installer = await readFile(path.join(root, "deploy/install.sh"), "utf8");
if (!installer.includes("\nSOURCE_REF=main\n"))
  throw Error("Missing installer revision marker");
await writeFile(
  path.join(dist, "install.sh"),
  installer.replace("\nSOURCE_REF=main\n", "\nSOURCE_REF=" + ref + "\n"),
);
await writeFile(path.join(dist, ".nojekyll"), "");
