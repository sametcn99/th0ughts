// Smoke-test the built site at its GitHub project path, including direct Turkish navigation.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium, expect } from "@playwright/test";
const dist = fileURLToPath(new URL("../pages/dist/", import.meta.url));
const mime = {
  ".css": "text/css",
  ".html": "text/html",
  ".js": "application/javascript",
  ".json": "application/json",
  ".jpg": "image/jpeg",
  ".webp": "image/webp",
  ".woff2": "font/woff2",
};
const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, "http://localhost");
    if (!url.pathname.startsWith("/th0ughts/")) throw Error();
    let file = path.resolve(dist, "." + url.pathname.slice("/th0ughts".length));
    if (
      !file.startsWith(path.resolve(dist) + path.sep) &&
      file !== path.resolve(dist)
    )
      throw Error();
    if ((await stat(file)).isDirectory()) file = path.join(file, "index.html");
    res.setHeader("Content-Type", mime[path.extname(file)] || "text/plain");
    res.end(await readFile(file));
  } catch {
    res.writeHead(404).end();
  }
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const origin = "http://127.0.0.1:" + server.address().port;
const browser = await chromium.launch();
try {
  for (const [lang, width] of [
    ["en", 1360],
    ["tr", 390],
  ]) {
    const ctx = await browser.newContext({
      viewport: { width, height: 900 },
      permissions: ["clipboard-read", "clipboard-write"],
    });
    const page = await ctx.newPage();
    const broken = [];
    page.on("pageerror", (e) => broken.push(e.message));
    page.on("response", (r) => {
      if (r.status() >= 400) broken.push(r.url());
    });
    await page.goto(origin + "/th0ughts/" + (lang === "tr" ? "tr/" : ""), {
      waitUntil: "networkidle",
    });
    await page.locator(".hero .shell button").click();
    assert.equal(
      await page.evaluate(() => navigator.clipboard.readText()),
      "curl -fsSL https://sametcn99.github.io/th0ughts/install.sh | bash",
    );
    await page.getByRole("button", { name: "Index", exact: true }).click();
    await expect(page.locator(".example img")).toHaveAttribute(
      "src",
      /theme-index.webp$/,
    );
    for (const button of await page.locator(".methods button").all())
      await button.click();
    await page
      .getByRole("button", { name: "Docker Compose", exact: true })
      .click();
    await page.locator(".guide-panel select").selectOption("mongodb");
    await expect(page.locator(".guide-panel pre")).toContainText(
      "compose.mongodb.yaml",
    );
    await page.locator(".more").click();
    const manifest = await (
      await page.request.get(origin + "/th0ughts/screenshots/manifest.json")
    ).json();
    await expect(page.locator(".card")).toHaveCount(manifest.length);
    for (const shot of manifest) {
      assert(
        (
          await page.request.get(origin + "/th0ughts/screenshots/" + shot.file)
        ).ok(),
        shot.file,
      );
    }
    await page.locator(".card").first().click();
    await expect(page.locator("dialog")).toBeVisible();
    await page.keyboard.press("ArrowRight");
    await expect(page.locator("dialog h3")).toHaveText(manifest[1].title[lang]);
    await page.keyboard.press("Escape");
    await expect(page.locator("dialog")).toBeHidden();
    await page.locator(".filters button").last().click();
    await expect(page.locator(".card")).toHaveCount(
      manifest.filter((s) => s.group === "storage").length,
    );
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth,
      ),
      false,
    );
    const themeGuide = page.locator("#handbook details").nth(1);
    await themeGuide.locator("summary").click();
    await expect(themeGuide.locator("code")).toContainText('content="violet"');
    await page.locator(".language").click();
    await expect(page.locator("html")).toHaveAttribute(
      "lang",
      lang === "en" ? "tr" : "en",
    );
    assert.deepEqual(broken, []);
    await ctx.close();
  }
  console.log("Pages passed English desktop and Turkish mobile checks.");
} finally {
  await browser.close();
  server.close();
}
