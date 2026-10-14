// Spike harness (#37): in Goose's approve mode, sends a message, allows the
// model's call once, and reports the view's console for a while after.
import { chromium } from "playwright";
const [shots, name, message, wait] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9333");
const page = browser.contexts()[0].pages()[0];
page.on("console", (m) => { const t = m.text(); if (t.includes("[spike]") && !t.includes("context changed")) console.log("console:", t.slice(0, 260)); });
const input = page.getByTestId("chat-input");
await input.fill(message);
await input.press("Enter");
const allow = page.getByRole("button", { name: /Allow Once/i });
await allow.first().waitFor({ timeout: 30000 });
console.log("approval asked for the model's call; allowing once");
await allow.first().click();
await page.waitForTimeout(Number(wait || 40000));
await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });
await browser.close();
