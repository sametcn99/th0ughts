import { test, expect } from "@playwright/test";
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";
import { createServer } from "node:https";
import { createHmac } from "node:crypto";
import { randomBytes } from "node:crypto";
import { mkdtemp, readFile, rm } from "node:fs/promises";
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
  const events = [];
  const hookSecret = randomBytes(32).toString("hex");
  const receiver = createServer({key: await readFile(mail.key), cert: await readFile(mail.cert)}, async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    const body = Buffer.concat(chunks);
    const timestamp = req.headers["x-th0ughts-timestamp"];
    const signature = "v1=" + createHmac("sha256", hookSecret).update(timestamp + ".").update(body).digest("hex");
    events.push({event:JSON.parse(body), valid: signature === req.headers["x-th0ughts-signature"] && Math.abs(Date.now()/1000 - Number(timestamp)) < 300});
    res.writeHead(204).end();
  });
  receiver.listen(0, "127.0.0.1");
  await once(receiver, "listening");
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
    const hook = settings.locator(".webhook-settings");
    await hook.locator("summary").click();
    await hook.getByLabel("Send comment events").check();
    await hook.getByLabel("Receiver URL (HTTPS)").fill("https://127.0.0.1:" + receiver.address().port);
    await hook.getByLabel("Signing secret").fill(hookSecret);
    await hook.getByRole("button", {name:"Save webhook"}).click();
    await expect(hook.locator(".notice")).toHaveText("Saved.");
    await expect(hook.getByLabel("Signing secret")).toHaveValue("");
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
    async function submit(email, body, images = false, requireCode = false) {
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
      if (requireCode) {
        await form
          .getByRole("button", { name: "Send verification code" })
          .click();
        await expect(form.getByLabel("Email verification code")).toBeVisible();
        expect(mail.codes.get(email)).toMatch(/^\d{6}$/);
        await form
          .getByLabel("Email verification code")
          .fill(mail.codes.get(email));
      } else {
        await expect(form.getByRole("button", { name: "Send verification code" })).toHaveCount(0);
      }
      await form.getByRole("button", { name: "Submit comment" }).click();
    }
    const photo = await (
      await import("node:fs/promises")
    ).readFile("scripts/fixtures/great-wave.jpg");
    await thread.getByRole("button", { name: "Add a comment" }).click();
    await submit("reader@iana.org", "A **guest** note.", true);
    await expect(thread.getByRole("status")).toContainText(
      "Submitted for approval",
    );
    expect(mail.codes.size).toBe(0);
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
    await settings.getByLabel("Require a code sent by email").check();
    await settings.getByLabel("Guest comments").selectOption("open");
    await settings.getByRole("button", { name: "Save", exact: true }).click();
    await expect(settings.locator(".settings-footer .notice")).toHaveText(
      "Saved.",
    );
    await page.reload();
    await thread.getByRole("button", { name: "Reply", exact: true }).click();
    await submit("another@iana.org", "A nested reply", false, true);
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
    await expect.poll(() => events.length).toBe(4);
    expect(events.every(x => x.valid)).toBe(true);
    expect(new Set(events.map(x => x.event.id)).size).toBe(4);
    expect(events.map(x => [x.event.type,x.event.data.status])).toEqual([
      ["comment.created","pending"], ["comment.approved","approved"], ["comment.created","approved"], ["comment.deleted","deleted"]
    ]);
    expect(events[2].event.data.parent).toBe(root.id);
    expect(events[0].event.data.images).toHaveLength(2);
    expect(events[3].event.data).not.toHaveProperty("body");
    expect(JSON.stringify(events)).not.toContain("reader@iana.org");
    expect(await api("/api/v1/get_comment_webhook")).toMatchObject({hasSecret:true,last:{state:"delivered",code:204}});
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
    receiver.closeAllConnections();
    await new Promise(resolve => receiver.close(resolve));
    await mail.close();
    await rm(dir, { recursive: true, force: true });
  }
});
