// Spike harness (#37): screenshots a client over CDP and prints its visible text.
import { chromium } from "playwright";
const [port, shot] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:" + port);
const pages = browser.contexts().flatMap((c) => c.pages());
const page = pages.find((p) => !p.url().startsWith("devtools")) || pages[0];
console.log("url:", page.url());
await page.screenshot({ path: shot });
console.log((await page.evaluate(() => document.body.innerText)).slice(0, 1500));
await browser.close();
