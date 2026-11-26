import { test, expect } from "@playwright/test";
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { once } from "node:events";
import { createInterface } from "node:readline";

async function listeningPort(stderr) {
  const lines = createInterface({ input: stderr });
  for await (const line of lines) {
    const match = /^Listening on 127\.0\.0\.1:(\d+)$/.exec(line);
    if (match) return Number(match[1]);
    console.error(line);
  }
  throw new Error("The server exited before binding its HTTP listener");
}

async function expectRepositoryFooter(page, lang) {
  const label = lang === "en" ? "host your own th0ughts" : "th0ughts’u kendi sunucunda çalıştır";
  const link = page.getByRole("contentinfo").getByRole("link", { name: label, exact: true });
  await expect(link).toBeVisible();
  await expect(link).toHaveAttribute("href", "https://github.com/sametcn99/th0ughts");
}

for (const [lang, mobile] of [
  ["en", false],
  ["tr", true],
]) {
  test("Owner and anonymous publishing in " + lang, async ({ browser }) => {
    const dir = await mkdtemp(path.join(tmpdir(), "thoughts-browser-"));
    const master = randomBytes(32).toString("hex");
    const app = spawn(
      process.env.TEST_BINARY || "bin/th0ughts",
      ["-listen", "127.0.0.1:0", "-data", dir, "serve"],
      {
        env: { ...process.env, TH0UGHTS_MASTER_KEY: master },
        stdio: ["ignore", "ignore", "pipe"],
      },
    );
    const closed = new Promise(resolve => app.once("close", resolve));
    let ctx;
    try {
      await once(app, "spawn");
      // Keep localhost as the WebAuthn origin, and probe the exact bound interface.
      const port = await listeningPort(app.stderr);
      const base = "http://localhost:" + port;
      ctx = await browser.newContext({
        baseURL: base,
        viewport: mobile
          ? { width: 390, height: 844 }
          : { width: 1280, height: 800 },
      });
      const health = await ctx.request.get("http://127.0.0.1:" + port + "/healthz");
      expect(health.status()).toBe(200);
      const page = await ctx.newPage();
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await page.goto("/?lang=" + lang);
      await expectRepositoryFooter(page, lang);
      await expect(page.locator('a[href*="/login"]')).toHaveCount(0);
      const cdp = await ctx.newCDPSession(page);
      const discovered = new Map();
      cdp.on("WebMCP.toolsAdded", ({ tools }) =>
        tools.forEach((tool) => discovered.set(tool.name, tool)),
      );
      await cdp.send("WebMCP.enable");
      await expect.poll(() => discovered.has("get_settings")).toBe(true);
      expect(discovered.has("save_post")).toBe(false);
      const tool = discovered.get("get_settings");
      const answer = new Promise((resolve) =>
        cdp.once("WebMCP.toolResponded", resolve),
      );
      await cdp.send("WebMCP.invokeTool", {
        frameId: tool.frameId,
        toolId: tool.toolId,
        input: {},
      });
      const invoked = await answer;
      expect(invoked.status).toBe("Completed");
      expect(JSON.stringify(invoked.output)).toContain("th0ughts");
      await page.goto("/login?lang=" + lang);
      await expectRepositoryFooter(page, lang);
      await page.locator("input[type=password]").fill(master);
      await page.locator("form button").first().click();
      await expect(
        page.locator("#composer input[type=password]"),
      ).toBeVisible();
      await page
        .locator("#composer input[type=password]")
        .fill("a-local-browser-password");
      await page.locator("#composer button").click();
      const newPost = page.getByRole("button", {
        name: lang === "en" ? "New post" : "Yeni gönderi",
      });
      await newPost.click();
      await page.locator("textarea").fill("A garden in İstanbul");
      await page
        .getByRole("button", {
          name: lang === "en" ? "Publish" : "Yayımla",
          exact: true,
        })
        .click();
      await expect(page.locator(".prose")).toHaveText("A garden in İstanbul");
      await page.locator("[data-edit]").click();
      await expect(page.locator("textarea")).toHaveValue(
        "A garden in İstanbul",
      );
      await page
        .getByRole("button", {
          name: lang === "en" ? "Türkçe" : "English",
          exact: true,
        })
        .click();
      await page.locator("textarea").fill("Private translated draft");
      await page
        .getByRole("button", {
          name: lang === "en" ? "Save draft" : "Taslak kaydet",
        })
        .click();
      await expect(newPost).toBeVisible();
      await page
        .getByRole("button", {
          name: lang === "en" ? "Settings" : "Ayarlar",
          exact: true,
        })
        .click();
      await page
        .getByRole("button", { name: lang === "en" ? "Appearance" : "Görünüm" })
        .click();
      if (!mobile) {
        await page.getByLabel("Reading font").selectOption("monospace");
        await page
          .getByRole("button", { name: "Preview", exact: true })
          .click();
        const previewPost = page
          .frameLocator("iframe")
          .locator(".prose")
          .first();
        await expect(previewPost).toHaveCSS("font-family", "monospace");
        await expect(page.locator(".prose").first()).not.toHaveCSS(
          "font-family",
          "monospace",
        );
        await page
          .getByRole("button", { name: "Preview", exact: true })
          .click();
      }
      await page
        .getByLabel(lang === "en" ? "Active theme" : "Etkin tema")
        .selectOption("index");
      await page
        .locator("#settings")
        .getByRole("button", {
          name: lang === "en" ? "Save" : "Kaydet",
          exact: true,
        })
        .click();
      await expect(page.locator(".index-layout")).toBeVisible();
      const publicCtx = await browser.newContext({
        baseURL: base,
        javaScriptEnabled: false,
        viewport: { width: 390, height: 844 },
      });
      const reader = await publicCtx.newPage();
      await reader.goto("/?lang=" + (lang === "en" ? "tr" : "en"));
      await expectRepositoryFooter(reader, lang === "en" ? "tr" : "en");
      await expect(reader.locator(".prose")).toHaveText("A garden in İstanbul");
      await expect(reader.locator("#settings")).toHaveCount(0);
      await expect(reader.locator('meta[property="og:title"]')).toHaveCount(1);
      expect(await reader.content()).not.toContain("Private translated draft");
      await publicCtx.close();
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth,
      );
      expect(overflow).toBe(false);
      if (!mobile) {
        // Upload the exact format accepted from a repository contributor.
        const source = (version) =>
          '<meta name="theme" content="folio" title="Folio">' +
          '<template id="item"><header class="folio-' +
          version +
          '"><theme-text en="Field notes" tr="Saha notları"></theme-text><slot name="title"></slot><slot name="byline"></slot></header><slot name="body"></slot><slot name="media"></slot><slot name="tags"></slot><slot name="actions"></slot></template>';
        async function appearance() {
          await page
            .getByRole("button", { name: "Settings", exact: true })
            .click();
          await page
            .getByRole("button", { name: "Appearance", exact: true })
            .click();
          await expect(page.getByLabel("Active theme").locator('option[value="paper"]')).toHaveCount(1);
        }
        async function uploadTheme(text) {
          const validated = page.waitForResponse(response =>
            new URL(response.url()).pathname === "/api/v1/preview_theme" && response.request().method() === "POST"
          );
          await page.getByLabel("Upload a theme").setInputFiles({
            name: "folio.html",
            mimeType: "text/html",
            buffer: Buffer.from(text),
          });
          expect((await validated).ok()).toBe(true);
          await expect(page.locator("#settings .notice")).toHaveText("Saved.");
          await expect(page.getByLabel("Active theme")).toHaveValue("folio");
        }
        await appearance();
        await uploadTheme(source("first"));
        await page
          .getByRole("button", { name: "Preview", exact: true })
          .click();
        const frame = page.frameLocator("iframe");
        await expect(frame.locator(".folio-first")).toContainText(
          "Field notes",
        );
        await expect(frame.locator("#state")).toHaveCount(0);
        await page.getByLabel("Sample content").check();
        await expect(frame.locator(".prose")).toHaveText(
          "A sample post for previewing this theme.",
        );
        await page.getByRole("button", { name: "Save", exact: true }).click();
        await expect(page.locator(".folio-first")).toBeVisible();
        await appearance();
        await uploadTheme(source("second"));
        const visitor = await browser.newContext({ baseURL: base });
        const publicPage = await visitor.newPage();
        await publicPage.goto("/");
        await expect(publicPage.locator(".folio-first")).toBeVisible();
        await page
          .getByRole("button", { name: "Preview", exact: true })
          .click();
        await expect(frame.locator(".folio-second")).toBeVisible();
        await visitor.close();
        await expect(page.locator("#settings .notice")).toHaveText("Saved.");
        await page.getByRole("button", { name: "Save", exact: true }).click();
        await expect(page.locator(".folio-second")).toBeVisible();
        await appearance();
        await page
          .getByRole("button", { name: "Restore previous theme version" })
          .click();
        await expect(page.locator("#settings .notice")).toHaveText("Saved.");
        await page.reload();
        await expect(page.locator(".folio-first")).toBeVisible();
        await cdp.send("WebAuthn.enable");
        await cdp.send("WebAuthn.addVirtualAuthenticator", {
          options: {
            protocol: "ctap2",
            transport: "internal",
            hasResidentKey: true,
            hasUserVerification: true,
            isUserVerified: true,
            automaticPresenceSimulation: true,
          },
        });
        await page
          .getByRole("button", { name: "Settings", exact: true })
          .click();
        await page
          .getByRole("button", { name: "Security", exact: true })
          .click();
        await page.getByRole("button", { name: "Register a passkey" }).click();
        await expect(page.locator("#settings .notice")).toHaveText("Saved.");
        await page.getByRole("button", { name: "Sign out" }).click();
        await expect(page.locator("#composer")).toHaveCount(0);
        await page.goto("/login");
        await page.getByRole("button", { name: "Use a passkey" }).click();
        await expect(newPost).toBeVisible();
      }
      expect(errors).toEqual([]);
    } finally {
      app.kill();
      await closed;
      await ctx?.close();
      await rm(dir, { recursive: true, force: true });
    }
  });
}
