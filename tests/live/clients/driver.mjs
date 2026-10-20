// Test only (ADR-036): operates a client's window over CDP for the client
// tests, one command a run, and prints what it found as JSON on stdout:
//
//   node driver.mjs <command> '<json arguments>'
//
// Every command takes the client ("goose" or "vscode") and the debugging port
// it was started with. The Go tests start the client, seed Mattermost and
// decide what each answer means; this only types, clicks and reads. Every wait
// is on a condition, with a deadline, never a fixed time.
import { chromium } from "playwright-core";

const [command, raw] = process.argv.slice(2);
const args = JSON.parse(raw || "{}");
const deadline = Date.now() + (args.timeout ?? 90_000);
let browser;

// until polls check until it answers something truthy, or fails at the
// deadline saying what it waited for.
async function until(what, check, every = 250) {
  let last;
  while (Date.now() < deadline) {
    try {
      const found = await check();
      if (found) return found;
    } catch (error) {
      last = error;
    }
    await new Promise((resolve) => setTimeout(resolve, every));
  }
  throw new Error(`timed out waiting for ${what}${last ? `: ${last.message}` : ""}`);
}

// mainPage is the client's window: Goose's only page, VS Code's workbench.
async function mainPage(browser) {
  return until("the client's window", () => {
    const pages = browser.contexts().flatMap((context) => context.pages());
    if (args.client === "vscode") return pages.find((page) => page.url().startsWith("vscode-file"));
    return pages.find((page) => !page.url().startsWith("devtools"));
  });
}

// frameText is a frame's visible text, with that of the frames inside it from
// its own origin, or "" when it cannot be read. VS Code draws a view in such a
// frame inside its webview's, which Playwright does not list.
async function frameText(frame) {
  try {
    return await frame.evaluate(() => {
      const texts = [];
      const walk = (doc) => {
        if (!doc) return;
        texts.push(doc.body ? doc.body.innerText : "");
        for (const inner of doc.querySelectorAll("iframe")) {
          try {
            walk(inner.contentDocument);
          } catch {
            // Another origin's frame, which Playwright lists itself.
          }
        }
      };
      walk(document);
      return texts.join("\n");
    });
  } catch {
    return "";
  }
}

// clickButton clicks the first button whose text matches name, in the frame or
// a frame inside it from its own origin, as a person's click would land.
async function clickButton(frame, name) {
  const clicked = await frame.evaluate((pattern) => {
    const wanted = new RegExp(pattern, "i");
    const find = (doc) => {
      if (!doc) return null;
      const button = [...doc.querySelectorAll("button")].find((b) => wanted.test(b.innerText));
      if (button) return button;
      for (const inner of doc.querySelectorAll("iframe")) {
        try {
          const found = find(inner.contentDocument);
          if (found) return found;
        } catch {
          // Another origin's frame.
        }
      }
      return null;
    };
    const button = find(document);
    if (!button) return false;
    button.click();
    return true;
  }, name);
  if (!clicked) throw new Error(`no button matches ${name}`);
}

// viewFrame is the frame below the window whose text holds text. Not finding
// it, it says which frames it saw.
async function viewFrame(page, text) {
  try {
    return await until(`a view that shows ${JSON.stringify(text)}`, async () => {
      for (const frame of page.frames()) {
        if (frame === page.mainFrame()) continue;
        if ((await frameText(frame)).includes(text)) return frame;
      }
      return null;
    });
  } catch (error) {
    const seen = [];
    for (const frame of page.frames()) {
      if (frame !== page.mainFrame()) seen.push(`${frame.url().slice(0, 80)}: ${JSON.stringify((await frameText(frame)).slice(0, 120))}`);
    }
    const session = await browser.newBrowserCDPSession();
    const { targetInfos } = await session.send("Target.getTargets");
    const targets = targetInfos.map((t) => `${t.type} ${t.url.slice(0, 70)} attached=${t.attached}`);
    throw new Error(`${error.message}; the frames: ${seen.join(" | ") || "none"}; the targets: ${targets.join(" | ")}`);
  }
}

// chatText is the conversation's text, without what views show.
async function chatText(page) {
  return page.evaluate(() => document.body.innerText);
}

function count(text, word) {
  return text.split(word).length - 1;
}

const clients = {
  goose: {
    async ready(page) {
      await until("Goose's chat input", async () => page.getByTestId("chat-input").isVisible());
    },
    async type(page, message) {
      const input = page.getByTestId("chat-input");
      await input.fill(message);
      await input.press("Enter");
    },
  },
  vscode: {
    async ready(page) {
      await until("VS Code's workbench", async () => page.locator(".monaco-workbench").isVisible());
      // Past the welcome, when it shows, without signing in.
      await until("the welcome to be gone", async () => {
        const overlay = page.locator(".onboarding-a-overlay.visible");
        if (!(await overlay.count())) return true;
        const choice = overlay.getByText(/Continue without Signing In|Skip|Continue|Next|Done|Get Started|Close/).last();
        if (await choice.count()) await choice.click();
        else await page.keyboard.press("Escape");
        return false;
      }, 1000);
      await runCommand(page, "Chat: Open Chat");
      await until("the chat's input", async () => page.locator(".interactive-input-editor").first().isVisible());
      // A model an extension offers is listed once the models have been
      // managed, without signing in: open that, and close it again.
      await runCommand(page, "Chat: Manage Language Models");
      const modal = page.locator(".monaco-modal-editor-block");
      await until("the language models", async () => modal.isVisible());
      await until("the language models to close", async () => {
        if (!(await modal.count())) return true;
        await page.keyboard.press("Escape");
        return false;
      }, 500);
      await pickModel(page);
      await startServer(page);
    },
    // type replaces whatever the input holds with message, inserted whole, so
    // the editor closes no bracket or quote of its own.
    async type(page, message) {
      await page.keyboard.press("Escape");
      const editor = page.locator(".interactive-input-editor").first();
      await editor.click();
      await page.keyboard.press("Control+A");
      await page.keyboard.press("Delete");
      await page.keyboard.insertText(message);
      await page.keyboard.press("Enter");
    },
  },
};

// runCommand runs a VS Code command through the command palette.
// A command an extension contributes is listed only once the extension host
// has started, so the query is typed again until the command shows.
async function runCommand(page, name) {
  const input = page.locator(".quick-input-widget input");
  const row = page.locator(".quick-input-widget .monaco-list-row", { hasText: name }).first();
  await until(`the command ${name}`, async () => {
    if (await row.isVisible()) return true;
    if (!(await input.isVisible())) {
      await page.keyboard.press("Escape");
      await page.keyboard.press("Control+Shift+P");
      return false;
    }
    await input.fill(">");
    await input.fill(">" + name);
    return false;
  }, 1000);
  await row.click();
}

// pickModel makes the scripted model the chat's model. The picker's label
// names the model it holds: "Models, Scripted model".
async function pickModel(page) {
  await until("the scripted model, picked", async () => {
    if (await page.locator('[aria-label^="Models, Scripted model"]').count()) return true;
    await page.keyboard.press("Escape");
    const picker = page.locator('[aria-label^="Models, "]').first();
    if (await picker.isVisible()) {
      await picker.click();
      const row = page.locator(".quick-input-widget .monaco-list-row, .context-view .monaco-list-row, .context-view .action-item").filter({ hasText: /Scripted model/ }).first();
      if (await row.isVisible()) await row.click();
    }
    return false;
  }, 1000);
}

async function startServer(page) {
  await runCommand(page, "MCP: List Servers");
  const server = page.locator(".quick-input-widget .monaco-list-row", { hasText: args.server }).first();
  await until(`the ${args.server} server in the list`, async () => server.isVisible());
  await server.click();
  const start = page.locator(".quick-input-widget .monaco-list-row", { hasText: /Start Server|Restart Server/ }).first();
  await until("the server's Start action", async () => start.isVisible());
  await start.click();
  await page.keyboard.press("Escape");
}

const commands = {
  // ready waits until the person could type a message to the scripted model.
  async ready(page) {
    await clients[args.client].ready(page);
    return {};
  },

  // send types a message, and waits for the model's answer to the tool's
  // result, the scripted model's Done: one more than the chat showed, or one
  // after the message sent. VS Code drops a turn scrolled out of sight from
  // the page, and the Done before it with it.
  async send(page) {
    const shown = await chatText(page);
    const before = count(shown, "Done.");
    const tail = args.message.slice(-24);
    const answered = (text) => {
      const at = text.lastIndexOf(tail);
      return at >= 0 && text.indexOf("Done.", at) > at;
    };
    const answeredBefore = answered(shown);
    await clients[args.client].type(page, args.message);
    if (args.approve) {
      // Goose asks with Allow Once, VS Code with Allow in this Session.
      // VS Code's is a link drawn as a button.
      const allow = page.locator("button, a.monaco-button").filter({ hasText: /Allow Once|Allow in this Session/i }).last();
      try {
        await until("the client's approval of the call", async () => allow.isVisible());
      } catch (error) {
        const seen = await page.evaluate(() => [...document.querySelectorAll("button, [role=button], a.monaco-button")]
          .filter((e) => e.getBoundingClientRect().height > 0)
          .map((e) => `${e.tagName}:${(e.getAttribute("aria-label") || e.innerText || "").trim().slice(0, 60)}`)
          .filter((label) => label.length > 4));
        throw new Error(`${error.message}; the buttons: ${seen.join(" | ")}`);
      }
      await allow.click();
    }
    await until("the model's Done after the tool answered", async () => {
      const text = await chatText(page);
      return count(text, "Done.") > before || (!answeredBefore && answered(text));
    });
    return {};
  },

  // view waits for a view that shows text, and answers with all it shows.
  async view(page) {
    const frame = await viewFrame(page, args.text);
    return { text: await frameText(frame) };
  },

  // press presses a button in the view that shows text, and waits until the
  // view shows until.
  async press(page) {
    const frame = await viewFrame(page, args.text);
    await clickButton(frame, args.button);
    await until(`the view to show ${JSON.stringify(args.until)}`, async () => (await frameText(frame)).includes(args.until));
    return { text: await frameText(frame) };
  },

  // views answers with the text of every frame below the window.
  async views(page) {
    const texts = [];
    for (const frame of page.frames()) {
      if (frame !== page.mainFrame()) texts.push(await frameText(frame));
    }
    return { texts };
  },

  async screenshot(page) {
    await page.screenshot({ path: args.path, fullPage: true });
    return {};
  },
};

if (!commands[command] || !clients[args.client]) {
  console.error(`usage: driver.mjs ${Object.keys(commands).join("|")} '{"client":"goose|vscode","port":…}'`);
  process.exit(2);
}
browser = await chromium.connectOverCDP(`http://127.0.0.1:${args.port}`);
try {
  console.log(JSON.stringify(await commands[command](await mainPage(browser))));
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
} finally {
  // Disconnects only: the client was started by the test, which ends it.
  await browser.close();
}
