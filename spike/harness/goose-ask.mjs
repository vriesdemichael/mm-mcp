// Spike harness (#37): has the view start a call that asks the person a
// question, in Goose desktop, and reports whether and where the question shows.
import { chromium } from "playwright";
const [shots, postID] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9333");
const page = browser.contexts()[0].pages()[0];
page.on("console", (m) => { const t = m.text(); if (t.includes("[spike]")) console.log("view console:", t.slice(0, 220)); });
const input = page.getByTestId("chat-input");
await input.fill("SPIKE spike_show_post " + JSON.stringify({ post_id: postID, mode: "full", auto: true, ask: true }));
await input.press("Enter");
await page.waitForTimeout(15000);
await page.screenshot({ path: `${shots}/goose-ask.png`, fullPage: true });
const text = await page.evaluate(() => document.body.innerText);
const at = text.indexOf("Spike #37");
console.log("question text on the page:", at >= 0 ? text.slice(Math.max(0, at - 200), at + 300) : "not found");
for (const frame of page.frames()) {
  const t = await frame.evaluate(() => document.body && document.body.innerText).catch(() => "");
  if (t && t.includes("Spike #37")) console.log("question inside frame", frame.url().slice(0, 50));
}
await browser.close();
