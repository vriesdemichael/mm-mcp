// Spike harness (#37): picks the scripted model in VS Code's open chat.
import { chromium } from "playwright";
const [shots] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9334");
const page = browser.contexts()[0].pages().find((p) => p.url().startsWith("vscode-file")) || browser.contexts()[0].pages()[0];
await page.keyboard.press("Escape");
await page.waitForTimeout(500);
await page.locator(".chat-input-picker-label", { hasText: "Models" }).first().click();
await page.waitForTimeout(1500);
const rows = await page.evaluate(() => [...document.querySelectorAll(".quick-input-widget .monaco-list-row, .context-view .monaco-list-row, .context-view .action-item")].map((e) => e.innerText.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 30));
console.log("menu:", rows.join(" | "));
await page.screenshot({ path: `${shots}/vscode-models.png` });
const spike = page.locator(".quick-input-widget .monaco-list-row, .context-view .monaco-list-row, .context-view .action-item").filter({ hasText: /spike/i }).first();
if (await spike.count()) { await spike.click(); await page.waitForTimeout(1500); console.log("picked spike"); } else { await page.keyboard.press("Escape"); }
console.log("pickers now:", await page.locator(".chat-input-picker-label").allInnerTexts());
await browser.close();
