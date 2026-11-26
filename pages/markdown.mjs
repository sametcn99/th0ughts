// Build-time only. No parser or document body is bundled into the browser app.
import { cp, mkdir, readFile, readdir } from "node:fs/promises";
import path from "node:path";
import MarkdownIt from "markdown-it";
import GithubSlugger from "github-slugger";
import { repo } from "./content.js";

const md = new MarkdownIt({ html: false });

export async function loadDocuments(root, dist) {
  const directory = path.join(root, "docs");
  const docs = [];
  async function visit(relative) {
    for (const entry of await readdir(path.join(directory, relative), {
      withFileTypes: true,
    })) {
      if (entry.name.startsWith(".")) continue;
      const file = path.posix.join(relative, entry.name);
      if (entry.isDirectory()) {
        await visit(file);
      } else if (entry.isFile() && /\.md$/i.test(file)) {
        const source = await readFile(path.join(directory, file), "utf8");
        const tokens = md.parse(source, {});
        const slugger = new GithubSlugger();
        const sections = [];
        let title;
        for (const [i, token] of tokens.entries()) {
          if (token.type !== "heading_open") continue;
          const label = tokens[i + 1].children
            .map((child) => child.content)
            .join("");
          const id = slugger.slug(label);
          token.attrSet("id", id);
          if (token.tag === "h1" && !title) title = label;
          if (token.tag === "h2") sections.push({ id, title: label });
        }
        docs.push({
          file,
          route:
            file.slice(0, -3).split("/").map(encodeURIComponent).join("/") +
            "/",
          title: title || entry.name.slice(0, -3),
          hasTitle: Boolean(title),
          sections,
          tokens,
        });
      } else if (entry.isFile()) {
        const target = path.join(dist, "docs", file);
        await mkdir(path.dirname(target), { recursive: true });
        await cp(path.join(directory, file), target);
      }
    }
  }
  await visit("");
  return docs.sort((a, b) => a.title.localeCompare(b.title, "en"));
}

// Resolve repository-relative Markdown links in the source document's directory.
// Both language routes share the same measurement files and images.
function destination(href, env) {
  if (!href || href.startsWith("#") || /^(?:[a-z][a-z\d+.-]*:|\/)/i.test(href))
    return href;
  const url = new URL(
    href,
    "https://source.invalid/docs/" + env.article.route.slice(0, -1) + ".md",
  );
  const suffix = url.search + url.hash;
  if (!url.pathname.startsWith("/docs/"))
    return repo + "/blob/main" + url.pathname + suffix;
  const source = url.pathname.slice("/docs/".length);
  const doc = env.routes.get(decodeURIComponent(source));
  return doc
    ? env.home + "docs/" + doc + suffix
    : env.prefix + "docs/" + source + suffix;
}

for (const [type, attribute] of [
  ["link_open", "href"],
  ["image", "src"],
]) {
  const original = md.renderer.rules[type];
  md.renderer.rules[type] = (tokens, idx, options, env, renderer) => {
    const token = tokens[idx];
    const href = token.attrGet(attribute);
    token.attrSet(attribute, destination(href, env));
    try {
      return original
        ? original(tokens, idx, options, env, renderer)
        : renderer.renderToken(tokens, idx, options);
    } finally {
      token.attrSet(attribute, href);
    }
  };
}

export function renderDocument(article, routes, prefix, lang) {
  return md.renderer.render(article.tokens, md.options, {
    article,
    routes,
    prefix,
    home: prefix + (lang === "tr" ? "tr/" : ""),
  });
}
