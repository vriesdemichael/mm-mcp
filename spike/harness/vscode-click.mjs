// Spike harness (#37): clicks the first visible element with the given text in
// VS Code, waits, and saves a screenshot.
import { chromium } from "playwright";
const [shots, name, ...texts] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9334");
const page = browser.contexts()[0].pages().find((p) => p.url().startsWith("vscode-file")) || browser.contexts()[0].pages()[0];
for (const text of texts) {
  const target = page.getByText(text, { exact: true }).last();
  console.log(text, "found:", await target.count());
  if (await target.count()) await target.click();
  await page.waitForTimeout(1500);
}
await page.waitForTimeout(4000);
await page.screenshot({ path: `${shots}/${name}.png` });
await browser.close();
