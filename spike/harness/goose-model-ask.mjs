// Spike harness (#37): has the model call a tool that asks, in Goose desktop,
// and reports whether the question shows and answers it if it does.
import { chromium } from "playwright";
const [shots] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9333");
const page = browser.contexts()[0].pages()[0];
const input = page.getByTestId("chat-input");
await input.fill("SPIKE spike_ask {}");
await input.press("Enter");
await page.waitForTimeout(10000);
await page.screenshot({ path: `${shots}/goose-model-ask.png`, fullPage: true });
const text = await page.evaluate(() => document.body.innerText);
const at = text.indexOf("Spike #37");
console.log("question:", at >= 0 ? text.slice(Math.max(0, at - 100), at + 400) : "not found");
const boxes = page.getByRole("checkbox");
console.log("checkboxes:", await boxes.count());
if (await boxes.count()) {
  await boxes.first().check();
  const buttons = await page.evaluate(() => [...document.querySelectorAll("button")].map((b) => b.innerText.trim()).filter(Boolean).slice(-12));
  console.log("buttons:", buttons.join(" | "));
  const submit = page.getByRole("button", { name: /submit|accept|send|confirm/i }).last();
  if (await submit.count()) { await submit.click(); await page.waitForTimeout(5000); }
  await page.screenshot({ path: `${shots}/goose-model-ask-answered.png`, fullPage: true });
}
await browser.close();
