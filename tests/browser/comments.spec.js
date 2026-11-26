import { test, expect } from "@playwright/test";
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";
import { randomBytes } from "node:crypto";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { once } from "node:events";
import { openCommentInbox } from "../../scripts/fixtures/comment-mail.mjs";

test("guest email, image moderation and nested replies", async ({
  browser,
}) => {
  const mail = await openCommentInbox(),
    dir = await mkdtemp(join(tmpdir(), "th0ughts-thread-")),
    key = randomBytes(32).toString("hex");
  const app = spawn(
    process.env.TEST_BINARY || "bin/th0ughts",
    ["-listen", "127.0.0.1:0", "-data", dir, "serve"],
    {
      env: {
        ...process.env,
        SSL_CERT_FILE: mail.cert,
        TH0UGHTS_MASTER_KEY: key,
      },
      stdio: ["ignore", "ignore", "pipe"],
    },
  );
  const closed = once(app, "close");
  let guest, owner;
  try {
    let baseURL;
    for await (const line of createInterface({ input: app.stderr })) {
      const m = line.match(/Listening on 127\.0\.0\.1:(\d+)/);
      if (m) {
        baseURL = "http://localhost:" + m[1];
        break;
      }
    }
    expect(baseURL).toBeTruthy();
    owner = await browser.newContext({ baseURL });
    guest = await browser.newContext({
      baseURL,
      viewport: { width: 390, height: 844 },
    });
    guest.setDefaultTimeout(6000);
    owner.setDefaultTimeout(6000);
    const api = async (path, data = {}) => {
      const r = await owner.request.post(path, {
        data,
        headers: { "X-Th0ughts-Request": "1" },
      });
      expect(r.ok(), path + " " + (await r.text())).toBeTruthy();
      return r.json();
    };
    await api("/auth/bootstrap", { master: key });
    await api("/auth/setup", { password: "fixture-password-123" });
    const post = await api("/api/v1/save_post", {
      original: "en",
      text: { en: { body: "A shared thread.", published: true } },
    });
    const panel = await owner.newPage();
    await panel.goto("/");
    await panel.getByRole("button", { name: "Settings", exact: true }).click();
    const settings = panel.getByRole("dialog");
    await settings
      .getByRole("button", { name: "Comments", exact: true })
      .click();
    await settings
      .getByText("Verification email delivery", { exact: true })
      .click();
    await settings.getByLabel("SMTP server").fill(mail.relay.host);
    await settings
      .getByLabel("Port", { exact: true })
      .fill(String(mail.relay.port));
    await settings.getByLabel("Connection security").selectOption("tls");
    await settings.getByLabel("Sender email address").fill(mail.relay.from);
    await settings.getByRole("button", { name: "Save mail settings" }).click();
    await expect(settings.locator(".mail-settings .notice")).toHaveText(
      "Saved.",
    );
    await settings.getByLabel("Guest comments").selectOption("review");
    await settings.getByLabel("Maximum images per comment").selectOption("2");
    await settings.getByLabel("Allow Markdown formatting").check();
    await settings.getByRole("button", { name: "Save", exact: true }).click();
    await expect(settings.locator(".settings-footer .notice")).toHaveText(
      "Saved.",
    );
    const page = await guest.newPage();
    await page.goto("/?post=" + post.id);
    const thread = page.locator("#comments");
    async function submit(email, body, images = false) {
      const form = thread.locator("form");
      await form.getByLabel("Your comment").fill(body);
      await form.getByLabel("Email (private)").fill(email);
      if (images) {
        await form
          .getByLabel("Format", { exact: true })
          .selectOption("markdown");
        await form
          .locator("input[type=file]")
          .setInputFiles(
            ["a", "b"].map((n) => ({
              name: n + ".jpg",
              mimeType: "image/jpeg",
              buffer: photo,
            })),
          );
      }
      await form
        .getByRole("button", { name: "Send verification code" })
        .click();
      await expect(form.getByLabel("Email verification code")).toBeVisible();
      expect(mail.codes.get(email)).toMatch(/^\d{6}$/);
      await form
        .getByLabel("Email verification code")
        .fill(mail.codes.get(email));
      await form.getByRole("button", { name: "Submit comment" }).click();
    }
    const photo = await (
      await import("node:fs/promises")
    ).readFile("scripts/fixtures/great-wave.jpg");
    await thread.getByRole("button", { name: "Add a comment" }).click();
    await submit("reader@example.com", "A **guest** note.", true);
    await expect(thread.getByRole("status")).toContainText(
      "Submitted for approval",
    );
    await expect(thread.locator(".thread-node")).toHaveCount(0);
    await panel.reload();
    await panel.getByRole("button", { name: "Settings", exact: true }).click();
    await settings
      .getByRole("button", { name: "Comments", exact: true })
      .click();
    const row = settings.locator(".moderation-entry");
    await expect(row.locator("img")).toHaveCount(2);
    const src = await row.locator("img").first().getAttribute("src");
    expect((await guest.request.get(src)).status()).toBe(404);
    await row.getByRole("button", { name: "Approve" }).click();
    await expect(row).toHaveCount(0);
    await page.reload();
    await expect(thread.locator(".thread-node strong").first()).toHaveText(
      "Unknown user",
    );
    expect((await guest.request.get(src)).status()).toBe(200);
    await settings.getByLabel("Guest comments").selectOption("open");
    await settings.getByRole("button", { name: "Save", exact: true }).click();
    await expect(settings.locator(".settings-footer .notice")).toHaveText(
      "Saved.",
    );
    await thread.getByRole("button", { name: "Reply", exact: true }).click();
    await submit("another@example.com", "A nested reply");
    await expect(thread.locator(".comment-branch")).toContainText(
      "A nested reply",
    );
    const reply = thread.locator(".comment-branch article").first();
    await reply.locator(".comment-meta a").first().click();
    await expect(
      thread.getByRole("link", { name: "All comments" }),
    ).toBeVisible();
    await page.reload();
    await expect(thread).toContainText("A nested reply");
    const root = (await api("/api/v1/list_comments", { post: post.id }))
      .items[0];
    await api("/api/v1/moderate_comment", { id: root.id, action: "delete" });
    expect((await guest.request.get(src)).status()).toBe(404);
    await page.goto("/?post=" + post.id);
    await thread.getByRole("button", { name: "Show replies" }).click();
    await expect(thread).toContainText("Deleted comment");
    await expect(thread).toContainText("A nested reply");
    await api("/api/v1/set_thread_lock", { id: post.id, closed: true });
    await page.reload();
    await expect(
      thread.getByRole("button", { name: "Add a comment" }),
    ).toHaveCount(0);
    await page.goto("/?lang=tr&post=" + post.id);
    await expect(thread).toContainText("Bu konuşma yalnızca okumaya açık.");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  } finally {
    await guest?.close();
    await owner?.close();
    app.kill("SIGTERM");
    await closed;
    await mail.close();
    await rm(dir, { recursive: true, force: true });
  }
});
