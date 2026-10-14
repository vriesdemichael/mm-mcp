// Spike harness (#37): sends one message in VS Code's open chat with the spike
// model already selected, waits, and reports frames, view console and chat text.
import { chromium } from "playwright";
const [shots, name, message, wait] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9334");
const page = browser.contexts()[0].pages().find((p) => p.url().startsWith("vscode-file")) || browser.contexts()[0].pages()[0];
for (const p of browser.contexts()[0].pages()) p.on("console", (m) => { const t = m.text(); if (t.includes("[spike]") && !t.includes("context changed")) console.log("console:", t.slice(0, 260)); });
await page.keyboard.press("Escape");
const editor = page.locator(".interactive-input-editor").first();
console.log("input found:", await editor.count(), "model:", await page.locator(".chat-input-picker-label").allInnerTexts());
await editor.click();
await page.keyboard.type(message);
await page.keyboard.press("Enter");
await page.waitForTimeout(Number(wait || 20000));
await page.screenshot({ path: `${shots}/${name}.png` });
const frames = [];
for (const f of page.frames()) frames.push(f.url().slice(0, 120));
console.log("frames:", frames.join("\n  "));
const boxes = await page.evaluate(() => [...document.querySelectorAll("iframe, webview")].map((f) => ({ tag: f.tagName, h: Math.round(f.getBoundingClientRect().height), w: Math.round(f.getBoundingClientRect().width), cls: String(f.className).slice(0, 50) })));
console.log("boxes:", JSON.stringify(boxes));
const chat = await page.evaluate(() => (document.querySelector(".interactive-session") || document.body).innerText);
console.log("chat text:", chat.slice(-1500));
await browser.close();
