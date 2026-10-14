// Spike harness (#37): opens "Chat: Manage Language Models" and reports it.
import { chromium } from "playwright";
const [shots, command] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9334");
const page = browser.contexts()[0].pages().find((p) => p.url().startsWith("vscode-file")) || browser.contexts()[0].pages()[0];
await page.keyboard.press("Escape");
await page.keyboard.press("Control+Shift+P");
await page.waitForTimeout(700);
await page.keyboard.type(command || "Manage Language Models");
await page.waitForTimeout(1200);
console.log("commands:", await page.evaluate(() => [...document.querySelectorAll(".quick-input-widget .monaco-list-row")].map((e) => e.innerText.replace(/\s+/g, " ")).slice(0, 8).join(" | ")));
await page.keyboard.press("Enter");
await page.waitForTimeout(3000);
await page.screenshot({ path: `${shots}/vscode-manage.png` });
console.log("editor:", await page.evaluate(() => (document.querySelector(".editor-group-container.active, .quick-input-widget") || document.body).innerText.replace(/\s+/g, " ").slice(0, 800)));
await browser.close();
