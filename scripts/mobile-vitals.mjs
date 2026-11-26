// Measure a real Chromium navigation under a fixed, reproducible mobile profile.
import fs from "node:fs/promises";
import { chromium } from "playwright";
const [url, output] = process.argv.slice(2);
if (!url || !output)
  throw Error("Usage: bun scripts/mobile-vitals.mjs URL OUTPUT");
const browser = await chromium.launch();
const results = [];
try {
  for (let i = 0; i < 8; i++) {
    const ctx = await browser.newContext({
      viewport: { width: 390, height: 844 },
      deviceScaleFactor: 2,
      isMobile: true,
      hasTouch: true,
    });
    // Install observers before page scripts so the initial paint is measured.
    const library = await fs.readFile(
      "node_modules/web-vitals/dist/web-vitals.iife.js",
      "utf8",
    );
    await ctx.addInitScript({
      content:
        library +
        ';window.metrics={}; for(const name of ["CLS","LCP","INP"]) webVitals["on"+name](m=>window.metrics[name]=m.value,{reportAllChanges:true,durationThreshold:16});',
    });
    const page = await ctx.newPage();
    const cdp = await ctx.newCDPSession(page);
    await cdp.send("Network.enable");
    await cdp.send("Network.emulateNetworkConditions", {
      offline: false,
      latency: 150,
      downloadThroughput: 1.6e6 / 8,
      uploadThroughput: 750000 / 8,
    });
    await cdp.send("Emulation.setCPUThrottlingRate", { rate: 4 });
    await page.goto(url + "/?lang=" + (i % 2 ? "tr" : "en"), {
      waitUntil: "networkidle",
    });
    await page.locator("input[type=search]").tap();
    await page.keyboard.type("garden", { delay: 35 });
    await page.evaluate(
      () =>
        new Promise((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(resolve)),
        ),
    );
    const m = await page.evaluate(() => ({
      ...window.metrics,
      interactions: performance.interactionCount,
      js: performance
        .getEntriesByType("resource")
        .filter((r) => r.name.includes(".js?"))
        .reduce((n, r) => n + r.encodedBodySize, 0),
    }));
    results.push(m);
    await ctx.close();
  }
} finally {
  await browser.close();
}
const pct = (rows, p) =>
  rows.sort((a, b) => a - b)[Math.ceil(p * rows.length) - 1];
const report = {
  browser: browser.version(),
  profile:
    "390x844, 4x CPU slowdown, 150ms RTT, 1.6 Mbps down / 750 Kbps up, eight fresh contexts",
  runs: results,
  p75: {},
};
for (const k of ["LCP", "CLS", "INP"]) {
  // Event Timing suppresses interactions below 16 ms. Use 16 as their upper bound.
  const vals = results.map(
    (m) =>
      m[k] ?? (k === "INP" && m.interactions > 0 ? 16 : k === "CLS" ? 0 : NaN),
  );
  if (vals.some((v) => !Number.isFinite(v)))
    throw Error("No usable " + k + " observations: " + JSON.stringify(results));
  report.p75[k] = pct(vals, 0.75);
}
await fs.writeFile(output, JSON.stringify(report, null, 2) + "\n");
console.log(report.p75);
if (
  report.p75.LCP > 2500 ||
  report.p75.INP > 200 ||
  report.p75.CLS > 0.1 ||
  results.some((m) => m.js > 100 * 1024)
)
  process.exitCode = 1;
