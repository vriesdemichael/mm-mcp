// Spike harness (#37): reopens the stored spike chat in VS Code through the
// agent sessions sidebar and reports whether its view renders again.
import { chromium } from "playwright";
const [shots] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9334");
const page = browser.contexts()[0].pages().find((p) => p.url().startsWith("vscode-file")) || browser.contexts()[0].pages()[0];
await page.keyboard.press("Escape");
await page.keyboard.press("Escape");
await page.keyboard.press("Control+Shift+P");
await page.waitForTimeout(700);
await page.keyboard.type("Show Agent Sessions Sidebar", { delay: 30 });
await page.waitForTimeout(1500);
await page.keyboard.press("Enter");
await page.waitForTimeout(3000);
await page.screenshot({ path: `${shots}/vscode-history-0.png` });
const entry = page.getByText(/SPIKE spike_show_post/).first();
console.log("stored chat found:", await entry.count());
if (await entry.count()) { await entry.click(); await page.waitForTimeout(8000); }
const boxes = await page.evaluate(() => [...document.querySelectorAll("iframe, webview")].map((f) => ({ tag: f.tagName, h: Math.round(f.getBoundingClientRect().height), w: Math.round(f.getBoundingClientRect().width) })));
console.log("boxes:", JSON.stringify(boxes));
await page.screenshot({ path: `${shots}/vscode-history-1.png` });
await browser.close();
