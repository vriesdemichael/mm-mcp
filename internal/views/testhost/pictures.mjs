// Never shipped with mm-mcp: pictures of the views for the README, which
// GitHub shows without running a script (mm-mcp#38). `task docs:view-pictures`
// runs it once the docs' view page, its host and the recordings are in
// docs/site/assets/views. Each view named below is mounted in the host, in
// GitHub's light and dark colours, and its frame photographed. A stamp of
// the page and the recordings they were made from lets a unit test say when
// the pictures are out of date.
//
//   node pictures.mjs <checkout>

import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { chromium } from "playwright-core";

const root = process.argv[2];
if (!root) {
  console.error("usage: node pictures.mjs <checkout>");
  process.exit(2);
}
const assets = join(root, "docs", "site", "assets", "views");
const out = join(assets, "pictures");

// The views the README shows, at the width a README column gives a picture.
const VIEWS = ["proposal", "thread"];
const WIDTH = 560;

// GitHub's own colours, so a picture sits in the README as if drawn there.
const THEMES = {
  light: {
    page: "#ffffff",
    variables: {
      "--color-background-primary": "#ffffff",
      "--color-background-secondary": "#f6f8fa",
      "--color-text-primary": "#1f2328",
      "--color-text-secondary": "#59636e",
      "--color-text-info": "#0969da",
      "--color-border-tertiary": "#d1d9e0",
      "--font-sans": "-apple-system, 'Segoe UI', 'Noto Sans', Helvetica, Arial, sans-serif",
    },
  },
  dark: {
    page: "#0d1117",
    variables: {
      "--color-background-primary": "#0d1117",
      "--color-background-secondary": "#151b23",
      "--color-text-primary": "#f0f6fc",
      "--color-text-secondary": "#9198a1",
      "--color-text-info": "#4493f8",
      "--color-border-tertiary": "#3d444d",
      "--font-sans": "-apple-system, 'Segoe UI', 'Noto Sans', Helvetica, Arial, sans-serif",
    },
  },
};

const sha256 = (data) => createHash("sha256").update(data).digest("hex");

const host = readFileSync(join(assets, "host.js"), "utf8");
const page = readFileSync(join(assets, "page.html"), "utf8");
const recordings = readdirSync(join(assets, "recordings")).filter((name) => name.endsWith(".json")).sort();

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: WIDTH + 40, height: 900 }, deviceScaleFactor: 2 });
  const tab = await context.newPage();
  mkdirSync(out, { recursive: true });
  for (const name of VIEWS) {
    const recording = JSON.parse(readFileSync(join(assets, "recordings", name + ".json"), "utf8"));
    for (const [theme, colours] of Object.entries(THEMES)) {
      await tab.setContent("<!doctype html><html><head><meta charset='utf-8'></head><body style='margin:0;padding:20px;background:" +
        colours.page + "'><div id='view' style='width:" + WIDTH + "px'></div></body></html>");
      await tab.addScriptTag({ content: host });
      await tab.evaluate(async ({ page, recording, theme, variables }) => {
        await window.mmHost.mount(page, {
          container: document.getElementById("view"),
          input: recording.input,
          result: recording.result,
          resources: recording.resources,
          theme,
          themes: { [theme]: variables },
        });
      }, { page, recording, theme, variables: colours.variables });
      // Drawn, sized, and every picture it reads arrived.
      await tab.waitForFunction(() => {
        const frame = window.mmHost.frames[0];
        if (!frame || !frame.ready || frame.sizes.length === 0) return false;
        const doc = frame.iframe.contentDocument;
        return doc && !doc.querySelector("[aria-busy]") && [...doc.images].every((img) => img.complete);
      }, null, { timeout: 30000 });
      await tab.waitForTimeout(300);
      await tab.locator("#view iframe").screenshot({ path: join(out, name + "-" + theme + ".png") });
    }
  }
} finally {
  await browser.close();
}

// The stamp: what the pictures were made from.
const stamp = ["page " + sha256(page)].concat(recordings.map((name) => "recording " + name + " " + sha256(readFileSync(join(assets, "recordings", name)))));
writeFileSync(join(out, "stamp.txt"), stamp.join("\n") + "\n");
console.log("pictures of " + VIEWS.join(", ") + " in " + out);
