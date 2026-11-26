import { test, expect } from "@playwright/test";
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { once } from "node:events";
import { createInterface } from "node:readline";

test.use({
  launchOptions: {
    args: ["--enable-experimental-web-platform-features", "--enable-features=WebMCP"],
  },
});

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
      const otherLang = lang === "en" ? "tr" : "en";
      const picker = page.locator(".language-picker");
      await page.goto("/?lang=" + lang + "&q=ink&tag=studio");
      await picker.locator("summary").focus();
      await page.keyboard.press("Enter");
      await expect(picker.locator("nav")).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(picker.locator("nav")).toBeHidden();
      await picker.locator("summary").click();
      await picker.locator('a[lang="' + otherLang + '"]').click();
      await expect(page.locator("html")).toHaveAttribute("lang", otherLang);
      expect(new URL(page.url()).searchParams.get("q")).toBe("ink");
      expect(new URL(page.url()).searchParams.get("tag")).toBe("studio");
      await page.goto("/?lang=" + lang);
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
      const newPost = page.getByPlaceholder(lang === "en" ? "What’s on your mind?" : "Aklından ne geçiyor?");
      await expect(newPost).toBeVisible();
      await expect(page.locator("#composer input[type=text]").first()).toBeHidden();
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
      await page.locator("#composer summary").click();
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
      const settings = page.getByRole("dialog", { name: lang === "en" ? "Settings" : "Ayarlar" });
      await expect(settings).toBeVisible();
      await expect(settings.getByRole("button", {name: lang === "en" ? "Save" : "Kaydet", exact: true})).toBeDisabled();
      await page.keyboard.press("Escape");
      const settingsTrigger = page.getByRole("button", {name: lang === "en" ? "Settings" : "Ayarlar", exact: true});
      await expect(settingsTrigger).toBeFocused();
      await settingsTrigger.click();
      if (mobile) {
        await settings.getByRole("combobox", { name: "Bölüm", exact: true }).selectOption("appearance");
        expect(await settings.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
      } else {
        await settings.getByRole("button", {name: "Appearance"}).click();
      }
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
      await expect(settings.locator(".notice")).toHaveText(lang === "en" ? "Saved." : "Kaydedildi.");
      await expect(settings.getByLabel(lang === "en" ? "Active theme" : "Etkin tema")).toHaveValue("index");
      await page.keyboard.press("Escape");
      const publicCtx = await browser.newContext({
        baseURL: base,
        javaScriptEnabled: false,
        viewport: { width: 390, height: 844 },
      });
      const reader = await publicCtx.newPage();
      await reader.goto("/?lang=" + (lang === "en" ? "tr" : "en"));
      await expectRepositoryFooter(reader, lang === "en" ? "tr" : "en");
      await expect(reader.locator(".prose")).toHaveText("A garden in İstanbul");
      await reader.locator(".byline a").click();
      const postId = new URL(reader.url()).searchParams.get("post");
      expect(postId).toBeTruthy();
      await reader.locator(".language-picker summary").click();
      await reader.locator('.language-picker a[lang="' + lang + '"]').click();
      await expect(reader.locator("html")).toHaveAttribute("lang", lang);
      expect(new URL(reader.url()).searchParams.get("post")).toBe(postId);
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
          '<style>:root { --surface:#faf3ed; --footer-surface:#deeedd; } @media (prefers-color-scheme: dark) { :root { --surface:#203040; } }</style>' +
          '<script id="options" type="application/json">[{"key":"gap","type":"range","min":0,"max":2,"step":0.25,"unit":"rem","default":"0.5","labels":{"en":"Gap","tr":"Aralık"}}]</script>' +
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
          await expect(page.locator("#settings .notice")).toHaveText("Theme ready to preview.");
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
        await expect(settings.getByLabel("Gap")).toHaveAttribute("step","0.25");
        const screen = settings.getByLabel("Preview screen");
        for (const [view, selector] of [["short",".entry-detail"],["article-list",".article-preview .read-post"],["article",".entry-detail h1"],["login","#login form"],["composer","#composer .compose-panel"],["settings","#settings dialog[open]"]]) {
          await screen.selectOption(view);
          await expect(frame.locator(selector)).toBeVisible();
          await expect(frame.locator("#state")).toHaveCount(0);
          await expect(frame.locator("html")).toHaveCSS("background-color","rgb(245, 247, 248)");
        }
        await expect(frame.locator("#settings dialog")).toHaveCSS("background-color","rgb(250, 243, 237)");
        await page.emulateMedia({colorScheme:"dark"});
        await expect(frame.locator("#settings dialog")).toHaveCSS("background-color","rgb(32, 48, 64)");
        await page.emulateMedia({colorScheme:"light"});
        await screen.selectOption("feed");
        await page.getByRole("button", { name: "Save", exact: true }).click();
        await expect(page.locator(".folio-first")).toBeVisible();
        await expect(settings.locator(".notice")).toHaveText("Saved.");
        await page.keyboard.press("Escape");
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
        await settings.getByRole("button",{name:"Site",exact:true}).click();
        const siteURL=settings.getByLabel("Public site URL");
        const originalURL=await siteURL.inputValue();
        await siteURL.fill("invalid");
        await settings.getByRole("button",{name:"Appearance",exact:true}).click();
        await settings.getByRole("button",{name:"Save",exact:true}).click();
        await expect(settings.getByRole("alert")).toHaveText("Check the information you entered.");
        await publicPage.reload();
        await expect(publicPage.locator(".folio-first")).toBeVisible();
        await expect(publicPage.locator(".site-footer")).toHaveCSS("background-color","rgb(222, 238, 221)");
        await settings.getByRole("button",{name:"Site",exact:true}).click();
        await siteURL.fill(originalURL);
        await settings.getByRole("button",{name:"Appearance",exact:true}).click();
        await visitor.close();
        await expect(page.locator("#settings .dirty-state")).toHaveText("Unsaved changes");
        await page.getByRole("button", { name: "Save", exact: true }).click();
        await expect(page.locator(".folio-second")).toBeVisible();
        await expect(settings.locator(".notice")).toHaveText("Saved.");
        await page.keyboard.press("Escape");
        await appearance();
        await page
          .getByRole("button", { name: "Restore previous theme version" })
          .click();
        await expect(page.locator("#settings .notice")).toHaveText("Saved.");
        await page.reload();
        await expect(page.locator(".folio-first")).toBeVisible();
        await appearance();
        await settings.getByRole("button",{name:"Version history"}).click();
        await expect(settings.getByLabel("Earlier revisions")).toHaveValue("0");
        const downloadEvent=page.waitForEvent("download");
        await settings.getByRole("button",{name:"Download theme file"}).click();
        const download=await downloadEvent;
        expect(download.suggestedFilename()).toBe("folio.html");
        await settings.getByLabel("Active theme").selectOption("paper");
        await settings.getByRole("button",{name:"Save",exact:true}).click();
        await expect(settings.locator(".notice")).toHaveText("Saved.");
        await settings.getByLabel("Active theme").selectOption("folio");
        page.once("dialog",d=>d.accept());
        await settings.getByRole("button",{name:"Delete theme",exact:true}).click();
        await expect(settings.locator('option[value="folio"]')).toHaveCount(0);
        await page.keyboard.press("Escape");
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

test("themed timeline appends cursor pages and retains crawlable links", async ({ browser }) => {
  const dir = await mkdtemp(path.join(tmpdir(), "thoughts-scroll-"));
  const master = randomBytes(32).toString("hex");
  const app = spawn(process.env.TEST_BINARY || "bin/th0ughts",
    ["-listen", "127.0.0.1:0", "-data", dir, "serve"],
    { env: { ...process.env, TH0UGHTS_MASTER_KEY: master }, stdio: ["ignore", "ignore", "pipe"] });
  const closed = once(app, "close");
  const contexts = [];
  try {
    const port = await listeningPort(app.stderr);
    const baseURL = "http://localhost:" + port;
    const owner = await browser.newContext({ baseURL });
    const guest = await browser.newContext({ baseURL, viewport: {width:390,height:740} });
    const crawler = await browser.newContext({ baseURL, javaScriptEnabled: false });
    contexts.push(owner, guest, crawler);
    const api = async (path, data) => {
      const r = await owner.request.post(path, { data, headers: {"X-Th0ughts-Request": "1"} });
      expect(r.ok(), await r.text()).toBeTruthy();
      return r.json();
    };
    await api("/auth/bootstrap", { master });
    await api("/auth/setup", { password: "scroll-test-password" });
    const settings = await api("/api/v1/get_settings", {});
    await api("/api/v1/save_settings", {...settings, theme: "index"});
    const published = [];
    let pinned, draft;
    for (let i = 0; i < 47; i++) {
      const p = await api("/api/v1/save_post", {
        original: "tr", pinned: i === 1, tags: [i === 45 ? "other" : "ink"],
        text: { tr: { body: i === 46 ? "Unrelated" : "Garden " + i, published: i !== 0 } },
      });
      if (i === 0) draft = p;
      if (i === 1) pinned = p;
      if (i >= 1 && i < 45) published.push(p.id);
    }
    const url = "/?lang=tr&q=Garden&tag=ink";
    const page = await guest.newPage();
    await page.goto(url);
    const posts = page.locator(".entries > [data-post-id]");
    await expect(posts).toHaveCount(20);
    await expect(posts.first()).toHaveAttribute("data-post-id", pinned.id);
    const older = page.locator("[data-feed-next]");
    const second = await older.getAttribute("href");

    const plain = await crawler.newPage();
    await plain.goto(url);
    await plain.locator("[data-feed-next]").click();
    await expect(plain).toHaveURL(baseURL + "/" + second);
    await expect(plain.locator(".entries > article")).toHaveCount(20);
    await expect(plain.locator('link[rel=canonical]')).toHaveAttribute("href", /cursor=/);

    // Move a previously seen pinned post behind the cursor between loads.
    await api("/api/v1/save_post", {...pinned, pinned:false});
    const requests = [];
    await page.route("**/*cursor=*", async route => {
      if (route.request().headers().accept !== "application/vnd.th0ughts.timeline+json") return route.continue();
      requests.push(route.request().url());
      if (requests.length === 1) return route.fulfill({status:503, body:""});
      return route.continue();
    });
    await posts.last().scrollIntoViewIfNeeded();
    await expect(older).toHaveText(/Tekrar dene/);
    await expect(posts).toHaveCount(20);
    await older.focus();
    await page.keyboard.press("Enter");
    await expect(posts).toHaveCount(40);
    expect(requests[0]).toBe(requests[1]);
    await posts.last().scrollIntoViewIfNeeded();
    await expect(posts).toHaveCount(44);
    await expect(older).toHaveCount(0);
    await expect(page).toHaveURL(baseURL + url);
    expect(requests).toHaveLength(3);
    for (const target of requests) {
      const params = new URL(target).searchParams;
      expect([params.get("lang"),params.get("tag"),params.get("q")]).toEqual(["tr","ink","Garden"]);
    }
    expect((await posts.evaluateAll(nodes => nodes.map(n => n.dataset.postId))).sort()).toEqual(published.sort());
    await expect(posts.last().locator("time").first()).toHaveAttribute("title", /.+/);
    await expect(page.locator("[data-edit]")).toHaveCount(0);

    const dashboard = await owner.newPage();
    await dashboard.goto(url);
    const entries = dashboard.locator(".entries > article");
    await entries.last().scrollIntoViewIfNeeded();
    await expect(entries).toHaveCount(40);
    await entries.last().scrollIntoViewIfNeeded();
    await expect(entries).toHaveCount(45);
    await dashboard.locator('[data-edit="' + draft.id + '"]').click();
    await expect(dashboard.locator("#composer textarea")).toHaveValue("Garden 0");
  } finally {
    for (const ctx of contexts) await ctx.close();
    app.kill();
    await closed;
    await rm(dir, {recursive:true,force:true});
  }
});
