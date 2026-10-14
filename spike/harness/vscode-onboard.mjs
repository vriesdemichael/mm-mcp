// Spike harness (#37): walks VS Code's welcome dialog without signing in.
import { chromium } from "playwright";
const [shots] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9334");
const page = browser.contexts()[0].pages().find((p) => p.url().startsWith("vscode-file")) || browser.contexts()[0].pages()[0];
for (let step = 0; step < 6; step++) {
  const overlay = page.locator(".onboarding-a-overlay.visible");
  if (!(await overlay.count())) { console.log("no overlay at step", step); break; }
  const text = await overlay.innerText();
  console.log(`step ${step}:`, text.replace(/\s+/g, " ").slice(0, 300));
  const buttons = await overlay.locator("button, a.monaco-button, .monaco-button").allInnerTexts();
  console.log("  buttons:", buttons.map((b) => b.trim()).filter(Boolean).join(" | "));
  const choice = overlay.getByText(/Continue without Signing In|Skip|Continue|Next|Done|Get Started|Close/).last();
  if (await choice.count()) { console.log("  pressing:", (await choice.innerText()).trim()); await choice.click(); }
  else { await page.keyboard.press("Escape"); }
  await page.waitForTimeout(2500);
}
await page.screenshot({ path: `${shots}/vscode-onboarded.png` });
await browser.close();
