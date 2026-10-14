// Spike harness (#37): opens VS Code's MCP server list and the spike server's
// actions, and reports them.
import { chromium } from "playwright";
const [shots] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9334");
const page = browser.contexts()[0].pages().find((p) => p.url().startsWith("vscode-file")) || browser.contexts()[0].pages()[0];
async function quick() { return page.evaluate(() => { const q = document.querySelector(".quick-input-widget"); return q && q.getBoundingClientRect().height > 0 ? q.innerText.replace(/\s+/g, " ").slice(0, 600) : ""; }); }
await page.keyboard.press("Control+Shift+P");
await page.waitForTimeout(700);
await page.keyboard.type("MCP: List Servers");
await page.waitForTimeout(1000);
await page.keyboard.press("Enter");
await page.waitForTimeout(2000);
console.log("servers:", await quick());
await page.keyboard.press("Escape");
await page.keyboard.press("Control+Shift+P");
await page.waitForTimeout(700);
await page.keyboard.type("MCP: List Servers");
await page.waitForTimeout(1000);
await page.keyboard.press("Enter");
await page.waitForTimeout(1500);
await page.keyboard.type("mmmcpspike");
await page.waitForTimeout(800);
await page.keyboard.press("Enter");
await page.waitForTimeout(2000);
console.log("actions:", await quick());
await page.keyboard.type("Start Server");
await page.waitForTimeout(800);
await page.keyboard.press("Enter");
await page.waitForTimeout(6000);
console.log("after start:", await quick());
const notes = await page.evaluate(() => [...document.querySelectorAll(".notification-toast, .monaco-dialog-box")].map((e) => e.innerText.replace(/\s+/g, " ").slice(0, 300)));
console.log("notifications:", notes.join(" || "));
await page.screenshot({ path: `${shots}/vscode-mcp.png` });
await browser.close();
