// Spike harness (#37): reports notifications, dialogs and chat questions
// visible in VS Code, and saves a screenshot.
import { chromium } from "playwright";
const [shots, name] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9334");
const page = browser.contexts()[0].pages().find((p) => p.url().startsWith("vscode-file")) || browser.contexts()[0].pages()[0];
const seen = await page.evaluate(() => [...document.querySelectorAll(".notification-toast, .notifications-center, .monaco-dialog-box, .quick-input-widget, .chat-elicitation-request, [class*=elicitation], [class*=confirmation]")]
  .filter((e) => e.getBoundingClientRect().height > 0).map((e) => e.className.slice(0, 50) + ": " + e.innerText.replace(/\s+/g, " ").slice(0, 300)));
console.log(seen.join("\n") || "nothing");
await page.screenshot({ path: `${shots}/${name}.png` });
await browser.close();
