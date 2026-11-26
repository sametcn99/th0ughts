// Capture the shipped application in a disposable instance, never a user's feed.
// Run from a checkout after building bin/th0ughts and installing Playwright Chromium.
import { spawn } from "node:child_process";
import { createHmac, randomBytes } from "node:crypto";
import { once } from "node:events";
import {
  access,
  mkdir,
  mkdtemp,
  readFile,
  rename,
  rm,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { createInterface } from "node:readline";
import { fileURLToPath } from "node:url";
import { chromium, expect } from "@playwright/test";


const root = fileURLToPath(new URL("../", import.meta.url));
const binary = process.env.TEST_BINARY || path.join(root, "bin/th0ughts");
const destination = path.join(root, "pages/public/screenshots");
const photo = path.join(root, "scripts/fixtures/great-wave.jpg");
const manifest = [];

async function listeningOrigin(stderr) {
  for await (const line of createInterface({ input: stderr })) {
    const match = /^Listening on 127\.0\.0\.1:(\d+)$/.exec(line);
    if (match) return "http://localhost:" + match[1];
    console.error(line);
  }
  throw new Error("The screenshot server exited before startup");
}

async function publish(api, body, extra = {}) {
  return api("save_post", {
    original: "en",
    text: { en: { body, published: true } },
    ...extra,
  });
}

async function seed(owner) {
  const request = owner.request;
  const headers = { "X-Th0ughts-Request": "1" };
  async function api(command, data = {}) {
    const response = await request.post("/api/v1/" + command, {
      headers,
      data,
    });
    if (!response.ok())
      throw new Error(command + " returned " + response.status());
    return response.json();
  }
  const site = await api("get_settings");
  await api("save_settings", {
    ...site,
    name: "Studio Notes",
    author: "Ada",
    description: "A notebook on print, color, and the walks in between.",
  });
  // Seed enough entries to exercise the real cursor and the second page.
  for (const color of [
    "Slate",
    "Rose",
    "Ochre",
    "Pine",
    "Chalk",
    "Coral",
    "Indigo",
    "Fern",
    "Ash",
    "Sand",
    "Olive",
    "Clay",
    "Copper",
    "Cobalt",
    "Plum",
    "Sage",
    "Amber",
    "Rust",
  ]) {
    await publish(
      api,
      color + " ink looks completely different once it dries.",
      { tags: ["swatches"] },
    );
  }
  await publish(
    api,
    "Back from the bindery. The new notebook opens completely flat. The smallest decisions are often the ones I notice every day.",
    { tags: ["paper", "studio"] },
  );
  await publish(
    api,
    "Leave one line unprinted. The paper does some of the drawing.",
    { tags: ["print", "studio"] },
  );
  const uploaded = await request.post("/api/v1/upload", {
    headers: { ...headers, "Content-Type": "image/jpeg" },
    data: await readFile(photo),
  });
  if (!uploaded.ok())
    throw new Error("Fixture upload returned " + uploaded.status());
  const media = await uploaded.json();
  const detail = await api("save_post", {
    original: "en",
    tags: ["print", "sketchbook"],
    media: [media.id],
    text: {
      en: {
        title: "A mountain between the waves",
        body: "I came back to Hokusai for the blue and found myself following the white. Fuji fits inside the curve of a single wave.\n\nI taped a small print above the workbench. From across the room the boats almost disappear. Up close, every hand has a job to do.\n\nTomorrow I want to try printing just the white space and see how much of the wave survives.",
        published: true,
        alt: {
          [media.id]: "Hokusai’s Great Wave, with Mount Fuji on the horizon",
        },
      },
      tr: {
        title: "Dalgaların arasındaki dağ",
        body: "Hokusai’nin mavisine dönüp beyaz çizgileri takip ettim. Fuji, tek bir dalganın kıvrımına sığıyor.\n\nKüçük bir baskıyı tezgâhın üzerine astım. Odanın öteki ucundan tekneler neredeyse kayboluyor. Yakından bakınca her bir elin ne yaptığı seçiliyor.\n\nYarın yalnızca beyaz boşlukları basıp dalgadan geriye ne kaldığına bakacağım.",
        published: true,
        alt: {
          [media.id]: "Hokusai’nin Büyük Dalga baskısı ve ufuktaki Fuji Dağı",
        },
      },
    },
  });
  const note = await api("save_post", {
    original: "en",
    pinned: true,
    text: {
      en: {
        body: "I keep the first print, even when the ink slips. It tells me what to change on the second one.",
        published: true,
      },
      tr: {
        body: "Mürekkep kaysa bile ilk baskıyı saklıyorum. İkincisinde neyi değiştireceğimi gösteriyor.",
        published: true,
      },
    },
  });
  await publish(
    api,
    "Left the sketchbook by the window. A sparrow did most of the posing today.",
  );
  const draft = await api("save_post", {
    original: "en",
    tags: ["notebook"],
    text: {
      en: {
        body: "Bring the small sketchbook next time.",
        published: false,
      },
    },
  });
  return { api, detail, draft, note, site: await api("get_settings") };
}

// Convert screenshots with the same Chromium that rendered them, so the capture
// toolchain needs no image converter or additional native dependencies.
function screenshotWriter(staging, encoder) {
  return async function shot(page, id, group, en, tr, options = {}) {
    if (manifest.some((entry) => entry.id === id))
      throw new Error("Duplicate screenshot: " + id);
    await page.evaluate(() => document.fonts.ready);
    const target = options.target || page;
    const png = await target.screenshot({
      type: "png",
      animations: "disabled",
      mask: options.mask || [],
      maskColor: "#697368",
    });
    const image = await encoder.evaluate(async (base64) => {
      const bytes = Uint8Array.from(atob(base64), (char) => char.charCodeAt(0));
      const bitmap = await createImageBitmap(
        new Blob([bytes], { type: "image/png" }),
      );
      const canvas = document.createElement("canvas");
      canvas.width = bitmap.width;
      canvas.height = bitmap.height;
      canvas.getContext("2d").drawImage(bitmap, 0, 0);
      bitmap.close();
      return canvas.toDataURL("image/webp", 0.9).split(",")[1];
    }, png.toString("base64"));
    const file = id + ".webp";
    await writeFile(path.join(staging, file), Buffer.from(image, "base64"));
    manifest.push({
      id,
      group,
      title: { en, tr },
      file,
      width: png.readUInt32BE(16),
      height: png.readUInt32BE(20),
    });
    console.log(file);
  };
}

const button = (scope, name) =>
  scope.getByRole("button", { name, exact: true });

async function navigate(page, url) {
  const response = await page.goto(url, { waitUntil: "networkidle" });
  if (!response?.ok())
    throw new Error("Navigation failed: " + url.split("?")[0]);
  await page.evaluate(() => document.fonts.ready);
}

async function captureAccessSetup(owner, reader, master, shot) {
  await navigate(reader, "/?lang=en");
  await shot(
    reader,
    "empty",
    "reader",
    "Starting an empty timeline",
    "Boş zaman akışı",
  );
  await navigate(owner, "/login?lang=en");
  await owner.getByLabel("Master key").waitFor();
  await shot(
    owner,
    "bootstrap",
    "access",
    "First sign-in with the master key",
    "Ana anahtarla ilk giriş",
  );
  await owner.getByLabel("Master key").fill(master);
  await button(owner, "Sign in").click();
  const setup = owner.locator("#composer");
  await setup.getByLabel("Database").waitFor();
  await shot(
    owner,
    "setup",
    "access",
    "Owner and database setup",
    "Sahip ve veritabanı kurulumu",
  );
  await setup.getByRole("combobox").selectOption("postgres");
  await setup.getByLabel("Database connection").waitFor();
  await shot(
    owner,
    "setup-database",
    "access",
    "Connecting an external database",
    "Harici veritabanına bağlantı",
    { target: setup },
  );
  await setup.getByRole("combobox").selectOption("sqlite");
  await setup.getByLabel("Password").fill(randomBytes(24).toString("hex"));
  await button(setup, "Set up your feed").click();
  await owner.getByPlaceholder("What’s on your mind?").waitFor();
  await navigate(reader, "/login?lang=en");
  await reader.getByLabel("Password").waitFor();
  await shot(
    reader,
    "login",
    "access",
    "Password, authenticator and passkey login",
    "Parola, doğrulayıcı ve geçiş anahtarıyla giriş",
  );
}

async function captureShortPost(reader, note, lang, shot) {
  const body = note.text[lang].body;
  await reader
    .locator("article")
    .filter({ hasText: body })
    .locator(".byline a")
    .click();
  await expect(reader.locator("article")).toHaveCount(1);
  await expect(reader.locator(".entry-detail .prose")).toHaveText(body);
  await expect(reader).toHaveURL(
    (url) =>
      url.searchParams.get("post") === note.id &&
      url.searchParams.get("lang") === lang,
  );
  const mobile = reader.viewportSize().width <= 650;
  await shot(
    reader,
    mobile ? "mobile-short-post" : "short-post",
    "reader",
    mobile ? "A short post on mobile" : "A short post at its permanent link",
    mobile ? "Mobilde kısa gönderi" : "Kalıcı bağlantısında kısa gönderi",
  );
}

async function captureReader(reader, { detail, note }, shot) {
  await navigate(reader, "/?lang=en");
  await reader
    .locator(".attachment")
    .first()
    .evaluate((img) => img.decode());
  await shot(
    reader,
    "timeline",
    "reader",
    "The public timeline",
    "Herkese açık zaman akışı",
  );
  const picker = reader.locator(".language-picker");
  await picker.locator("summary").click();
  await expect(picker.locator("nav")).toBeVisible();
  await shot(reader, "language", "reader", "Choosing the interface language", "Arayüz dilini seçme");
  await picker.locator("summary").click();
  await captureShortPost(reader, note, "en", shot);
  await navigate(reader, "/?lang=en");
  await reader.getByRole("link", { name: "Read article" }).click();
  await expect(reader.locator(".entry-detail h1")).toHaveText(
    detail.text.en.title,
  );
  await expect(reader.locator(".entry-detail .prose")).toContainText(
    "how much of the wave survives",
  );
  await reader.locator(".attachment").evaluate((img) => img.decode());
  await shot(
    reader,
    "post",
    "reader",
    "An article opened from the timeline",
    "Akıştan açılan bir yazı",
  );
  await navigate(reader, "/?q=bindery&lang=en");
  await expect(reader.locator("article")).toHaveCount(1);
  await shot(reader, "search", "reader", "Search results", "Arama sonuçları");
  await navigate(reader, "/?q=unprinted-query&lang=en");
  await expect(reader.locator("article")).toHaveCount(0);
  await shot(
    reader,
    "search-empty",
    "reader",
    "Search without matches",
    "Sonuçsuz arama",
  );
  await navigate(reader, "/?tag=studio&lang=en");
  await expect(reader.locator("article")).toHaveCount(2);
  await shot(
    reader,
    "tag",
    "reader",
    "Posts with the studio tag",
    "Studio etiketli gönderiler",
  );
  await navigate(reader, "/?lang=en");
  const older = await reader.locator("a.older").getAttribute("href");
  await navigate(reader, "/" + older);
  await shot(reader, "archive", "reader", "Older posts", "Önceki gönderiler");
}

async function capturePublishing(owner, detail, draft, shot) {
  await navigate(owner, "/?lang=en");
  await shot(
    owner,
    "owner",
    "publishing",
    "The owner's timeline",
    "Sahibin zaman akışı",
  );
  const form = owner.locator("#composer .panel");
  await form
    .getByLabel("Post text (Markdown)")
    .fill("The light moved across the studio wall while the ink dried.");
  await shot(
    owner,
    "compose",
    "publishing",
    "Writing a post",
    "Gönderi yazma",
    { target: form },
  );
  await button(form, "Cancel").click();
  await navigate(owner, "/?post=" + detail.id + "&lang=en");
  await button(owner, "Edit").click();
  await form.locator("summary").click();
  await expect(form.getByLabel("Title")).toHaveValue(detail.text.en.title);
  await shot(
    owner,
    "edit-media",
    "publishing",
    "Editing attachments and sharing metadata",
    "Ekleri ve paylaşım bilgilerini düzenleme",
    { target: form },
  );
  await button(form, "Türkçe").click();
  await expect(form.getByLabel("Title")).toHaveValue(detail.text.tr.title);
  await shot(
    owner,
    "translation",
    "publishing",
    "Editing a Turkish translation",
    "Türkçe çeviri düzenleme",
    { target: form },
  );
  await button(form, "Cancel").click();
  await navigate(owner, "/?post=" + draft.id + "&lang=en");
  await expect(owner.locator(".byline")).toContainText("Draft");
  await shot(owner, "draft", "publishing", "A private draft", "Özel taslak");
}

// RFC 6238, using the fresh temporary instance's setup secret only in memory.
function authenticatorCode(secret) {
  let bits = "";
  for (const c of secret.replace(/=+$/, "")) {
    const digit = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567".indexOf(c.toUpperCase());
    if (digit < 0) throw new Error("Unexpected authenticator secret format");
    bits += digit.toString(2).padStart(5, "0");
  }
  const key = Buffer.from(bits.match(/.{8}/g).map((byte) => parseInt(byte, 2)));
  const step = Buffer.alloc(8);
  step.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const digest = createHmac("sha1", key).update(step).digest();
  return String(
    (digest.readUInt32BE(digest[19] & 15) & 0x7fffffff) % 1000000,
  ).padStart(6, "0");
}

async function captureSettings(owner, shot) {
  await navigate(owner, "/?lang=en");
  await button(owner, "Settings").click();
  const panel = owner.locator("#settings .panel");
  async function tab(name) {
    await button(panel, name).click();
    await expect(button(panel, name)).toHaveAttribute("aria-pressed", "true");
  }
  const panelShot = (id, group, en, tr, options) =>
    shot(owner, id, group, en, tr, { target: panel, ...options });
  await panel.getByLabel("Public site URL").waitFor();
  await panelShot(
    "settings",
    "publishing",
    "Site profile and language settings",
    "Site profili ve dil ayarları",
  );
  await tab("Updates");
  await expect(panel.getByText("Running version", {exact:false})).toBeVisible();
  await panelShot("updates", "publishing", "Application updates", "Uygulama güncellemeleri");
  await tab("Appearance");
  await panel.getByLabel("Reading font").waitFor();
  await panelShot(
    "appearance",
    "appearance",
    "Theme, typography and spacing",
    "Tema, yazı tipi ve boşluklar",
  );
  await button(panel, "Preview").click();
  const frame = panel.frameLocator("iframe");
  await frame.locator("article").first().waitFor();
  await panelShot(
    "preview",
    "appearance",
    "Theme preview with published posts",
    "Yayınlanan gönderilerle tema önizlemesi",
  );
  await panel.getByLabel("Sample content").check();
  await panel.getByLabel("Owner", { exact: true }).check();
  await expect(frame.locator(".prose")).toContainText("sample post");
  await panelShot(
    "preview-owner",
    "appearance",
    "Sample content in owner preview",
    "Sahip önizlemesinde örnek içerik",
  );
  for (const [view, selector, en, tr] of [
    ["short", ".entry-detail", "Short post permalink preview", "Kısa gönderi önizlemesi"],
    ["article-list", ".article-preview", "Article card preview", "Makale kartı önizlemesi"],
    ["article", ".entry-detail h1", "Full Article preview", "Makale sayfası önizlemesi"],
    ["login", "#login form", "Sign-in preview", "Giriş ekranı önizlemesi"],
    ["composer", ".compose-panel", "Composer preview", "Düzenleyici önizlemesi"],
    ["settings", "dialog[open]", "Settings dialog preview", "Ayarlar penceresi önizlemesi"],
  ]) {
    await panel.getByLabel("Preview screen").selectOption(view);
    await frame.locator(selector).waitFor();
    await panel.locator("iframe").scrollIntoViewIfNeeded();
    await panelShot("preview-" + view, "appearance", en, tr);
  }
  await button(panel, "Preview").click();
  // Keep revision management visible in the gallery without changing the live theme.
  for (const title of ["Notebook One", "Notebook Two", "Notebook Three"]) {
    await owner.request.post("/api/v1/install_theme", {
      headers: {"X-Th0ughts-Request":"1"},
      data: {source:`<meta name="theme" content="notebook" title="${title}">`},
      failOnStatusCode:true,
    });
  }
  await tab("Site"); await tab("Appearance");
  await panel.getByLabel("Active theme").selectOption("notebook");
  await button(panel,"Version history").click();
  await panel.getByLabel("Earlier revisions").selectOption("1");
  await panelShot("theme-history","appearance","Theme downloads and revision history","Tema indirme ve sürüm geçmişi");
  await tab("Access tokens");
  await panel.getByLabel("Token name").fill("Notebook CLI");
  await panel.getByLabel("Write posts").check();
  await panelShot(
    "tokens",
    "access",
    "Scoped access token settings",
    "Yetki kapsamlı erişim anahtarı ayarları",
  );
  await button(panel, "Create").click();
  await panel.locator("output").waitFor();
  await panelShot(
    "token-issued",
    "access",
    "Issued token and revocation",
    "Oluşturulan erişim anahtarı ve iptali",
    { mask: [panel.locator("output")] },
  );
  await tab("Storage");
  await panel.getByText("Download backup").waitFor();
  await panelShot(
    "storage",
    "storage",
    "Local storage, backup and restore",
    "Yerel depolama, yedekleme ve geri yükleme",
  );
  await panel.getByRole("combobox").selectOption("S3");
  await panel.getByLabel("S3 endpoint").fill("objects.example.com");
  await panel.getByLabel("Bucket").fill("studio-notes");
  await panelShot(
    "storage-s3",
    "storage",
    "S3-compatible storage",
    "S3 uyumlu depolama",
  );
  await tab("Security");
  await panelShot(
    "security",
    "access",
    "Passkey and authenticator settings",
    "Geçiş anahtarı ve doğrulayıcı ayarları",
  );
  await button(panel, "Verify your identity").click();
  await panel.getByLabel("Password").waitFor();
  await panelShot(
    "reauth",
    "access",
    "Verify identity for sensitive actions",
    "Hassas işlemler için kimlik doğrulama",
  );
  await button(panel, "Verify your identity").click();
  await button(panel, "Set up authenticator").click();
  const secret = panel.locator("code");
  await secret.waitFor();
  await panelShot(
    "authenticator",
    "access",
    "Authenticator enrollment",
    "Doğrulayıcı kurulumu",
    { mask: [secret] },
  );
  await panel
    .getByLabel("Authenticator or recovery code")
    .fill(authenticatorCode(await secret.innerText()));
  await button(panel, "Save").click();
  const codes = panel.locator("output");
  await codes.waitFor();
  await panelShot(
    "recovery-codes",
    "access",
    "Single-use recovery codes",
    "Tek kullanımlık kurtarma kodları",
    { mask: [codes] },
  );
}

async function captureThemesAndMobile(owner, reader, data, shot) {
  for (const theme of ["index", "starter", "dusk"]) {
    await data.api("save_settings", { ...data.site, theme });
    await reader.emulateMedia({colorScheme:theme==="dusk"?"dark":"light"});
    await navigate(reader, "/?lang=en");
    await shot(
      reader,
      "theme-" + theme,
      "appearance",
      theme.charAt(0).toUpperCase() + theme.slice(1) + " theme",
      theme.charAt(0).toUpperCase() + theme.slice(1) + " teması",
    );
  }
  await reader.emulateMedia({colorScheme:"light"});
  await data.api("save_settings", data.site);
  await reader.setViewportSize({ width: 390, height: 844 });
  await navigate(reader, "/?lang=tr");
  await shot(
    reader,
    "mobile",
    "reader",
    "Mobile timeline in Turkish",
    "Türkçe mobil zaman akışı",
  );
  await captureShortPost(reader, data.note, "tr", shot);
  await navigate(reader, "/?lang=tr&post=" + data.detail.id);
  await shot(
    reader,
    "mobile-post",
    "reader",
    "Translated post on mobile",
    "Mobilde çevrilmiş gönderi",
  );
  await navigate(reader, "/login?lang=tr");
  await reader.locator("form").waitFor();
  await shot(
    reader,
    "mobile-login",
    "access",
    "Mobile sign-in in Turkish",
    "Türkçe mobil giriş",
  );
  await owner.setViewportSize({ width: 390, height: 844 });
  await navigate(owner, "/?lang=tr");
  await button(owner, "Ayarlar").click();
  const panel = owner.locator("#settings .panel");
  await panel.getByRole("combobox", { name: "Bölüm", exact: true }).selectOption("appearance");
  await panel.getByLabel("Etkin tema").waitFor();
  await shot(
    owner,
    "mobile-appearance",
    "appearance",
    "Appearance settings on mobile",
    "Mobilde görünüm ayarları",
    { target: panel },
  );
}

async function captureComments(owner,reader,data,shot){
  await data.api("save_settings",{...data.site,comments:{mode:"open",markdown:true,images:3}});
  const upload=await readFile(photo);
  async function submit(email,name,body,parent="",image=false){
    const multipart={email,name,body,format:"markdown",parent};
    if(image)multipart.image={name:"print.jpg",mimeType:"image/jpeg",buffer:upload};
    const res=await reader.request.post("/api/v1/comments/"+data.note.id,{multipart,failOnStatusCode:true});
    return res.json();
  }
  const root=await submit("reader@iana.org","Mira","The thin blue lines really stand out in this **print**.", "",true);
  await submit("reply@iana.org","Deniz","Especially along the edge of the wave.",root.id);
  await data.api("save_settings",{...data.site,comments:{mode:"review",markdown:true,images:3}});
  await submit("sarah@iana.org","Sarah","Which paper did you use for the second impression?");
  await reader.setViewportSize({width:1100,height:820});await navigate(reader,"/?lang=en&post="+data.note.id);
  const thread=reader.locator("#comments");await button(thread,"Show replies").click();
  await expect(thread.locator(".comment-branch")).toContainText("Deniz");
  await shot(reader,"comment-thread","reader","Threaded comments and images","Görselli yorumlar ve yanıtlar",{target:thread});
  await reader.setViewportSize({width:390,height:844});await navigate(reader,"/?post="+data.note.id+"&lang=tr");
  await button(thread,"Yanıtları göster").click();await expect(thread.locator(".comment-branch")).toContainText("Deniz");
  await shot(reader,"comment-mobile","reader","Reading replies on mobile","Mobilde yanıtları okuma",{target:thread});
  await button(thread,"Yorum yaz").click();
  await shot(reader,"guest-comment","reader","Guest comment form","Ziyaretçi yorum formu",{target:thread.locator("form")});
  await owner.setViewportSize({width:1120,height:860});await navigate(owner,"/?lang=en");await button(owner,"Settings").click();
  const panel=owner.locator("#settings .settings-dialog");await button(panel,"Comments").click();await expect(panel.locator(".moderation-entry")).toContainText("Sarah");
  const policy = panel.locator("fieldset").filter({has:owner.getByLabel("Require a code sent by email")});
  await shot(owner, "comment-verification", "publishing", "Guest comment verification policy", "Ziyaretçi yorumlarında e-posta doğrulama ayarı", {target:policy});
  const webhook = panel.locator(".webhook-settings");
  await webhook.locator("summary").click();
  await webhook.getByLabel("Send comment events").check();
  await webhook.getByLabel("Receiver URL (HTTPS)").fill("https://example.org/hooks/comments");
  await webhook.scrollIntoViewIfNeeded();
  await shot(owner, "comment-webhook", "publishing", "Signed comment webhook settings", "İmzalı yorum webhook ayarları", {target:webhook});
  await webhook.locator("summary").click();
  await panel.locator(".moderation-entry").scrollIntoViewIfNeeded();await shot(owner,"comment-moderation","publishing","Comment settings and moderation queue","Yorum ayarları ve moderasyon kuyruğu",{target:panel});
}

async function captureAll(staging, dataDir) {
  const master = randomBytes(32).toString("hex");
  const app = spawn(
    binary,
    [...(process.platform === "linux" && ["x64","arm64"].includes(process.arch) ? ["-managed-updates"] : []), "-listen", "127.0.0.1:0", "-data", dataDir, "serve"],
    {
      env: { ...process.env, TH0UGHTS_MASTER_KEY: master },
      stdio: ["ignore", "ignore", "pipe"],
    },
  );
  const closed = new Promise((resolve) => app.once("close", resolve));
  let browser;
  try {
    await once(app, "spawn");
    const baseURL = await listeningOrigin(app.stderr);
    browser = await chromium.launch();
    const options = {
      baseURL,
      viewport: { width: 1120, height: 860 },
      reducedMotion: "reduce",
      locale: "en-US",
      timezoneId: "UTC",
    };
    const owner = await (await browser.newContext(options)).newPage();
    const reader = await (await browser.newContext(options)).newPage();
    owner.setDefaultTimeout(8000);
    reader.setDefaultTimeout(8000);
    const encoder = await browser.newPage();
    const shot = screenshotWriter(staging, encoder);
    await captureAccessSetup(owner, reader, master, shot);
    const data = await seed(owner);
    await captureReader(reader, data, shot);
    await capturePublishing(owner, data.detail, data.draft, shot);
    await captureSettings(owner, shot);
    await captureThemesAndMobile(owner, reader, data, shot);
    await captureComments(owner,reader,data,shot);
    // Lead with the public feed everywhere the manifest is consumed.
    manifest.unshift(
      manifest.splice(
        manifest.findIndex((row) => row.id === "timeline"),
        1,
      )[0],
    );
    await writeFile(
      path.join(staging, "manifest.json"),
      JSON.stringify(manifest, null, 2) + "\n",
    );
  } finally {
    app.kill();
    await closed;
    await browser?.close();
  }
}

async function updateReadme() {
  const file = path.join(root, "README.md");
  const begin = "<!-- gallery:start -->",
    end = "<!-- gallery:end -->";
  const readme = await readFile(file, "utf8");
  const start = readme.indexOf(begin),
    finish = readme.indexOf(end);
  if ((start === -1) !== (finish === -1) || (finish !== -1 && finish < start))
    throw new Error("Invalid README gallery markers");
  const lines = [
    begin,
    "<details>",
    "<summary>Application screenshots (" + manifest.length + ")</summary>",
    "",
  ];
  lines.push(
    "Build the Go binary (see Local development), install Chromium with `bun run --bun playwright install chromium`, and run `bun run screenshots` to refresh this gallery. The script creates and removes its own demo instance.",
    "",
  );
  for (const row of manifest) {
    lines.push(
      "**" + row.title.en + "**",
      "",
      "![" +
        row.title.en +
        "](https://sametcn99.github.io/th0ughts/screenshots/" +
        row.file +
        ")",
      "",
    );
  }
  lines.push("</details>", end);
  const section = lines.join("\n");
  const next =
    start === -1
      ? readme.trimEnd() + "\n\n" + section + "\n"
      : readme.slice(0, start) + section + readme.slice(finish + end.length);
  await writeFile(file, next);
}

await access(binary).catch(() => {
  throw new Error(
    "Build bin/th0ughts with go build -o bin/th0ughts ./cmd/th0ughts or set TEST_BINARY.",
  );
});
await mkdir(path.dirname(destination), { recursive: true });
const dataDir = await mkdtemp(path.join(tmpdir(), "th0ughts-gallery-data-"));
const staging = await mkdtemp(
  path.join(path.dirname(destination), ".screenshots-"),
);
try {
  await captureAll(staging, dataDir);
  await mkdir(destination, { recursive: true });
  for (const { file } of manifest)
    await rename(path.join(staging, file), path.join(destination, file));
  await rename(
    path.join(staging, "manifest.json"),
    path.join(destination, "manifest.json"),
  );
  await updateReadme();
  console.log("Captured " + manifest.length + " application views.");
} finally {
  await rm(staging, { recursive: true, force: true });
  await rm(dataDir, { recursive: true, force: true });
}
