// Spike harness (#37): drives the MCP Apps reference client (basic-host) in
// headless Chromium: calls spike_show_post in each mode and reports what the
// view drew, its size, and the view's own log.
import { chromium } from "playwright";
const postID = process.argv[2];
const shots = process.argv[3];
// CHROMIUM names a browser to use instead of the one Playwright installed.
const browser = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
const page = await browser.newPage({ viewport: { width: 1100, height: 1400 } });
page.on("console", (m) => { if (m.text().includes("[spike]") || m.text().toLowerCase().includes("context update")) console.log("console:", m.text()); });
await page.goto("http://localhost:8080/index.html");
const tool = page.locator("select").nth(1);
await tool.waitFor();
await page.waitForFunction(() => [...document.querySelectorAll("select")].some((s) => [...s.options].some((o) => o.value === "spike_show_post")), null, { timeout: 30000 });
for (const mode of ["full", "line", "none"]) {
  await tool.selectOption("spike_show_post");
  await page.locator("textarea").fill(JSON.stringify({ post_id: postID, mode, auto: mode === "full", force_views: true }));
  await page.getByRole("button", { name: "Call Tool" }).click();
  await page.waitForTimeout(6000);
  const frames = page.locator("iframe");
  const count = await frames.count();
  const last = frames.nth(count - 1);
  const box = await last.boundingBox();
  console.log(`mode ${mode}: ${count} iframes; last iframe box ${JSON.stringify(box)}`);
  await page.screenshot({ path: `${shots}/basic-host-${mode}.png`, fullPage: true });
}
// The view's text, through the double frame: outer sandbox frame, inner srcdoc frame.
for (const frame of page.frames()) {
  const text = await frame.evaluate(() => document.body && document.body.innerText).catch(() => "");
  if (text && text.includes("@")) console.log("frame", frame.url().slice(0, 40), "text:\n" + text.slice(0, 1500));
}
await browser.close();
