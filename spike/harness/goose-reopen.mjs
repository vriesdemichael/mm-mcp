// Spike harness (#37): opens each earlier conversation in Goose desktop's
// sidebar and reports whether its views render again.
import { chromium } from "playwright";
const [shots] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9333");
const page = browser.contexts()[0].pages()[0];
const marks = await page.evaluate(() => {
  const found = [];
  for (const el of document.querySelectorAll("*")) {
    if (el.children.length === 0 && /^spike model/.test(el.textContent.trim())) {
      const r = el.getBoundingClientRect();
      if (r.left < 240 && r.width > 0) found.push({ x: r.left + r.width / 2, y: r.top + r.height / 2 });
    }
  }
  return found;
});
console.log("sidebar entries:", marks.length);
for (let i = 0; i < marks.length; i++) {
  await page.mouse.click(marks[i].x, marks[i].y);
  await page.waitForTimeout(8000);
  const info = await page.evaluate(() => ({
    iframes: [...document.querySelectorAll("iframe")].map((f) => Math.round(f.getBoundingClientRect().height)),
    toolCards: (document.body.innerText.match(/Spike Show Post/g) || []).length,
  }));
  console.log(`conversation ${i}: ${JSON.stringify(info)}`);
  if (info.toolCards) await page.screenshot({ path: `${shots}/goose-reopened-${i}.png`, fullPage: true });
}
await browser.close();
