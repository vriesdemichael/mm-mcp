// Spike harness (#37): sends one message in Goose desktop's chat, waits, and
// reports the view's console, frames and visible approval prompts.
import { chromium } from "playwright";
const [shots, name, message, wait] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9333");
const page = browser.contexts()[0].pages()[0];
page.on("console", (m) => { const t = m.text(); if (t.includes("[spike]") && !t.includes("context changed")) console.log("console:", t.slice(0, 220)); });
const input = page.getByTestId("chat-input");
await input.fill(message);
await input.press("Enter");
await page.waitForTimeout(Number(wait || 12000));
const boxes = await page.evaluate(() => [...document.querySelectorAll("iframe")].map((f) => { const r = f.getBoundingClientRect(); return { w: Math.round(r.width), h: Math.round(r.height) }; }));
console.log("iframes:", JSON.stringify(boxes));
const buttons = await page.evaluate(() => [...document.querySelectorAll("button")].map((b) => b.innerText.trim()).filter((t) => /allow|deny|approve|always/i.test(t)));
console.log("approval buttons:", buttons.join(" | ") || "none");
await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });
await browser.close();
