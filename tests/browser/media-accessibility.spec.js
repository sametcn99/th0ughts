import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createInterface } from "node:readline";

let app, stopped, directory, baseURL, owner, article, note;
async function api(path, data) {
  const response = await owner.request.post(path, {
    data,
    headers: { "X-Th0ughts-Request": "1" },
  });
  expect(response.ok(), await response.text()).toBeTruthy();
  return response.json();
}
test.beforeAll(async ({ browser }) => {
  directory = await mkdtemp(join(tmpdir(), "th0ughts-media-"));
  const master = randomBytes(32).toString("hex");
  app = spawn(
    process.env.TEST_BINARY || "bin/th0ughts",
    ["-listen", "127.0.0.1:0", "-data", directory, "serve"],
    {
      env: { ...process.env, TH0UGHTS_MASTER_KEY: master },
      stdio: ["ignore", "ignore", "pipe"],
    },
  );
  stopped = once(app, "close");
  for await (const line of createInterface({ input: app.stderr })) {
    const match = line.match(/Listening on 127\.0\.0\.1:(\d+)/);
    if (match) {
      baseURL = "http://localhost:" + match[1];
      break;
    }
  }
  expect(baseURL, "Server must announce its listening address").toBeTruthy();
  owner = await browser.newContext({ baseURL });
  const setup = await owner.newPage();
  await setup.goto("/login");
  await expect(setup.getByLabel("Master key")).toBeVisible();
  expect(await audit(setup, "first sign-in")).toEqual([]);
  await api("/auth/bootstrap", { master });
  await setup.goto("/");
  await expect(
    setup.getByRole("combobox", { name: "Database", exact: true }),
  ).toBeVisible();
  expect(await audit(setup, "owner setup")).toEqual([]);
  await setup.close();
  await api("/auth/setup", { password: "fixture-password-123" });
  const files = [
    ["jpeg", await readFile("scripts/fixtures/great-wave.jpg")],
    [
      "gif",
      Buffer.from(
        "R0lGODlhAQABAIEAAAAA/wAAAAAAAAAAACH/C05FVFNDQVBFMi4wAwEAAAAh+QQAeAAAACwAAAAAAQABAAAIBAABBAQAIfkEAXgAAQAsAAAAAAEAAQCB/wAAAAAAAAAAAAAACAQAAQQEADs=",
        "base64",
      ),
    ],
    [
      "png",
      Buffer.from(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNgYPgPAAEDAQAIicLsAAAAAElFTkSuQmCC",
        "base64",
      ),
    ],
  ];
  const media = [],
    alt = {};
  for (const [format, data] of files) {
    const res = await owner.request.post("/api/v1/upload", {
      data,
      headers: { "Content-Type": "image/" + format, "X-Th0ughts-Request": "1" },
    });
    expect(res.ok(), await res.text()).toBeTruthy();
    const { id } = await res.json();
    media.push(id);
    alt[id] = format + " attachment";
  }
  const body = [
    "From idea to print.",
    "",
    "```mermaid",
    "flowchart LR",
    "accTitle: Print workflow",
    "A[Draft] --> B[Publish]",
    "```",
    "",
    "![Print plate](/media/" + media[0] + ")",
    "",
    "```mermaid",
    "invalid chart",
    "```",
  ].join("\n");
  article = await api("/api/v1/save_post", {
    original: "en",
    media,
    text: { en: { title: "Print workflow", body, published: true, alt } },
  });
  note = await api("/api/v1/save_post", {
    original: "en",
    text: { en: { body: "A short plain note.", published: true } },
  });
});
test.afterAll(async () => {
  await owner?.close();
  if (app && app.exitCode === null) app.kill();
  await stopped;
  if (directory) await rm(directory, { recursive: true, force: true });
});

async function audit(page, label) {
  const { violations } = await new AxeBuilder({ page })
    .withTags([
      "wcag2a",
      "wcag2aa",
      "wcag21a",
      "wcag21aa",
      "wcag22aa",
      "best-practice",
    ])
    .analyze();
  return violations.map((v) => ({
    screen: label,
    rule: v.id,
    targets: v.nodes.slice(0, 3).map((n) => [n.target, n.failureSummary]),
  }));
}

test(
  "diagram rendering, fallback, original media and keyboard focus",
  { tag: "@media" },
  async ({ page, browser }) => {
    const requests = [];
    page.on("request", (r) => requests.push(r.url()));
    await page.goto(baseURL + "/?post=" + note.id);
    expect(requests.some((url) => url.includes("diagram-library"))).toBe(false);
    await page.goto(baseURL + "/?post=" + article.id);
    const trigger = page.getByRole("button", {
      name: /Enlarge Print workflow/,
    });
    await expect(trigger).toBeVisible({ timeout: 16000 });
    await expect(page.locator(".diagram-worker")).toHaveAttribute(
      "sandbox",
      "allow-scripts",
    );
    expect(
      await page
        .locator(".diagram-worker")
        .evaluate((frame) => frame.contentDocument),
    ).toBeNull();
    expect(
      (
        await page.request.get(baseURL + "/post/" + article.id + ".md")
      ).status(),
    ).toBe(200);
    await trigger.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toHaveAccessibleName("Print workflow");
    await expect(dialog.getByRole("button", { name: "Close" })).toBeFocused();
    await page.keyboard.press("+");
    await expect(dialog.locator("output")).toHaveText("150%");
    expect(await audit(page, "viewer")).toEqual([]);
    await page.keyboard.press("Escape");
    await expect(trigger).toBeFocused();
    await page.getByText("Diagram source", { exact: true }).click();
    await expect(page.locator(".mermaid-figure pre")).toContainText("A[Draft]");
    await page.locator("code").last().scrollIntoViewIfNeeded();
    await expect(page.locator(".diagram-error")).toBeVisible();
    for (const format of ["jpeg", "png", "gif"]) {
      const link = page.getByRole("link", { name: format + " attachment" });
      const src = await link.getAttribute("href");
      await link.click();
      await expect(dialog.locator("img")).toHaveJSProperty(
        "naturalWidth",
        format === "jpeg" ? 600 : 1,
      );
      expect(
        new URL(await dialog.locator("img").getAttribute("src")).pathname,
      ).toBe(src);
      await dialog.getByRole("button", { name: "Close" }).click();
      await expect(link).toBeFocused();
    }
    await page.getByRole("button", { name: "Enlarge Print plate" }).click();
    await expect(dialog).toHaveAccessibleName("Print plate");
    await page.keyboard.press("Escape");
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.getByRole("link", { name: "gif attachment" }).click();
    const animation = dialog.getByRole("button", { name: "Animate GIF" });
    await expect(animation).toHaveAttribute("aria-pressed", "false");
    await expect(dialog.locator("img")).toHaveAttribute("src", /\?w=960$/);
    await expect(dialog.locator("img")).toHaveJSProperty("naturalWidth", 1);
    await animation.click();
    await expect(animation).toHaveAttribute("aria-pressed", "true");
    await expect(dialog.locator("img")).not.toHaveAttribute("src", /\?w=/);
    await page.keyboard.press("Escape");
    const ctx = await browser.newContext({ javaScriptEnabled: false });
    const plain = await ctx.newPage();
    await plain.goto(baseURL + "/?post=" + article.id);
    await expect(plain.locator("code").first()).toContainText("flowchart LR");
    await ctx.close();
  },
);

test("touch pinch, pan, lock, fit and mobile reflow", async ({ browser }) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
  });
  const page = await context.newPage();
  await page.goto(baseURL + "/?lang=tr&post=" + article.id);
  await page.getByRole("link", { name: "jpeg attachment" }).tap();
  const dialog = page.getByRole("dialog"),
    stage = dialog.locator(".viewer-stage"),
    img = stage.locator("img");
  await expect(img).toHaveJSProperty("complete", true);
  const box = await stage.boundingBox(),
    cx = box.x + box.width / 2,
    cy = box.y + box.height / 2;
  const cdp = await context.newCDPSession(page);
  async function touch(type, points) {
    await cdp.send("Input.dispatchTouchEvent", {
      type,
      touchPoints: points.map(([x, y], id) => ({ x, y, id })),
    });
  }
  await touch("touchStart", [
    [cx - 24, cy],
    [cx + 24, cy],
  ]);
  await touch("touchMove", [
    [cx - 84, cy],
    [cx + 84, cy],
  ]);
  await touch("touchEnd", []);
  await expect
    .poll(async () => parseInt(await dialog.locator("output").textContent()))
    .toBeGreaterThan(200);
  const pos = () => img.evaluate((el) => el.style.transform);
  const start = await pos();
  await touch("touchStart", [[cx, cy]]);
  await touch("touchMove", [[cx + 80, cy]]);
  await touch("touchEnd", []);
  expect(await pos()).not.toBe(start);
  const pin = dialog.getByRole("button", {
    name: "Görünümü sabitle",
    pressed: false,
  });
  const lockBox = await pin.boundingBox();
  const lockPoint = [
    [lockBox.x + lockBox.width / 2, lockBox.y + lockBox.height / 2],
  ];
  await touch("touchStart", lockPoint);
  await touch("touchEnd", []);
  await expect(
    dialog.getByRole("button", { name: "Görünümü sabitle" }),
  ).toHaveAttribute("aria-pressed", "true");
  const locked = await pos();
  await touch("touchStart", [[cx, cy]]);
  await touch("touchMove", [[cx - 80, cy]]);
  await touch("touchEnd", []);
  expect(await pos()).toBe(locked);
  await dialog.getByRole("button", { pressed: true }).tap();
  await expect(pin).toBeVisible();
  await page.keyboard.press("0");
  await expect(dialog.locator("output")).toHaveText("100%");
  await page.setViewportSize({ width: 320, height: 568 });
  expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(
    true,
  );
  await context.close();
});

test("accessibility across reader, login, composer and every settings section", async ({
  browser,
}) => {
  test.setTimeout(180000);
  const guest = await browser.newContext({ baseURL });
  const page = await guest.newPage(),
    failures = [];
  for (const [label, path] of [
    ["feed", "/"],
    ["login", "/login"],
    ["detail", "/?post=" + note.id],
    ["empty search", "/?q=unmatched"],
  ]) {
    await page.goto(path);
    if (label === "login")
      await expect(page.locator("input[type=password]")).toBeVisible();
    failures.push(...(await audit(page, label)));
  }
  const panel = await owner.newPage();
  await panel.goto("/");
  await panel.locator(".compose-options summary").click();
  failures.push(...(await audit(panel, "composer")));
  await panel.getByRole("button", { name: "Settings", exact: true }).click();
  for (const button of await panel.locator(".settings-nav button").all()) {
    await button.click();
    failures.push(...(await audit(panel, await button.textContent())));
  }
  await panel.keyboard.press("Escape");
  await expect(
    panel.getByRole("button", { name: "Settings", exact: true }),
  ).toBeFocused();
  await panel.setViewportSize({ width: 320, height: 640 });
  await panel.goto("/?lang=tr");
  await panel.locator(".settings-launch button").click();
  const section = panel.locator(".section-select select");
  for (const value of await section
    .locator("option")
    .evaluateAll((items) => items.map((o) => o.value))) {
    await section.selectOption(value);
    expect(
      await panel
        .locator(".settings-content")
        .evaluate((el) => el.scrollWidth <= el.clientWidth),
      "mobile " + value,
    ).toBe(true);
    failures.push(...(await audit(panel, "mobile " + value)));
  }
  await panel.close();
  await guest.close();
  expect(failures).toEqual([]);
});

test("mobile reader, guest comment form and bundled theme accessibility", async ({
  browser,
}) => {
  test.setTimeout(180000);
  const initial = await api("/api/v1/get_settings", {});
  const themes = await api("/api/v1/list_themes", {});
  const ctx = await browser.newContext({
    baseURL,
    viewport: { width: 320, height: 640 },
    hasTouch: true,
    colorScheme: "dark",
    reducedMotion: "reduce",
  });
  const page = await ctx.newPage(),
    failures = [];
  try {
    for (const { id } of themes) {
      await api("/api/v1/save_settings", {
        ...initial,
        theme: id,
        comments: { ...initial.comments, mode: "review" },
      });
      await page.goto("/?lang=tr");
      await page.keyboard.press("Tab");
      const skip = page.locator(".skip-content");
      await expect(skip).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(page.locator("#feed-content")).toBeFocused();
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth),
        id,
      ).toBeLessThanOrEqual(320);
      failures.push(...(await audit(page, id)));
    }
    await page.goto("/?lang=en&post=" + note.id);
    const add = page.getByRole("button", { name: "Add a comment" });
    await add.click();
    await expect(
      page.getByRole("textbox", { name: "Your comment" }),
    ).toBeFocused();
    failures.push(...(await audit(page, "guest comment")));
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(add).toBeFocused();
  } finally {
    await api("/api/v1/save_settings", initial);
    await ctx.close();
  }
  expect(failures).toEqual([]);
});

test(
  "diagrams in appended timeline pages render on approach",
  { tag: "@media" },
  async ({ page }) => {
    for (let i = 0; i < 20; i++) {
      await api("/api/v1/save_post", {
        original: "en",
        text: { en: { body: "Later note " + i, published: true } },
      });
    }
    await page.goto(baseURL);
    const anchor = page.locator("a[data-feed-next]");
    await expect(anchor).toHaveCount(1);
    await anchor.scrollIntoViewIfNeeded();
    const card = page.locator(
      `[data-scroll-item][data-post-id="${article.id}"]`,
    );
    await card.scrollIntoViewIfNeeded();
    await card.getByRole("button", { name: /Enlarge Print workflow/ }).click();
    await expect(page.getByRole("dialog")).toHaveAccessibleName(
      "Print workflow",
    );
  },
);

test(
  "multiple diagram types open in a narrow Turkish reader",
  { tag: "@media" },
  async ({ browser, browserName }) => {
    const body = [
      "```mermaid",
      "sequenceDiagram",
      "  Kaynak->>Okur: Baskı hazır",
      "```",
      "",
      "```mermaid",
      "pie title Renkler",
      '  "Mavi" : 3',
      '  "Kırmızı" : 2',
      "```",
    ].join("\n");
    const sample = await api("/api/v1/save_post", {
      original: "tr",
      text: { tr: { body, published: true } },
    });
    const context = await browser.newContext({
      baseURL,
      viewport: { width: 390, height: 844 },
      hasTouch: true,
      isMobile: browserName !== "firefox",
    });
    try {
      const page = await context.newPage();
      await page.goto("/?post=" + sample.id + "&lang=tr");
      for (const i of [0, 1]) {
        const figure = page.locator(".mermaid-figure").nth(i);
        await page
          .locator("code.language-mermaid")
          .nth(i)
          .evaluate((el) => el.closest("figure,pre").scrollIntoView());
        const trigger = figure.getByRole("button");
        await trigger.tap();
        const dialog = page.getByRole("dialog");
        await expect(dialog.locator("img")).toHaveJSProperty("complete", true);
        await dialog.getByRole("button", { name: "Yakınlaştır" }).tap();
        await expect(dialog.locator("output")).toHaveText("150%");
        await dialog.getByRole("button", { name: "Kapat" }).tap();
        await expect(trigger).toBeFocused();
      }
    } finally {
      await context.close();
    }
  },
);
