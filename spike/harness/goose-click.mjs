// Spike harness (#37): clicks the last button with the given text in Goose
// desktop, waits, and reports the approval buttons left.
import { chromium } from "playwright";
const [shots, name, text, wait] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9333");
const page = browser.contexts()[0].pages()[0];
page.on("console", (m) => { const t = m.text(); if (t.includes("[spike]") && !t.includes("context changed")) console.log("console:", t.slice(0, 220)); });
const target = page.getByRole("button", { name: text, exact: true }).last();
console.log(text, "found:", await target.count());
if (await target.count()) await target.click();
await page.waitForTimeout(Number(wait || 10000));
const buttons = await page.evaluate(() => [...document.querySelectorAll("button")].map((b) => b.innerText.trim()).filter((t) => /allow|deny|approve|always/i.test(t)));
console.log("approval buttons:", buttons.join(" | ") || "none");
const boxes = await page.evaluate(() => [...document.querySelectorAll("iframe")].map((f) => { const r = f.getBoundingClientRect(); return { w: Math.round(r.width), h: Math.round(r.height) }; }));
console.log("iframes:", JSON.stringify(boxes));
await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });
await browser.close();
