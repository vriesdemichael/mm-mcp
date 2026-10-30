// Pictures and emoji.
//
// The view has no network: a picture is one of mm-mcp's resources, which the
// host reads from mm-mcp and hands over as data. RESOURCE_BASE is where
// mm-mcp's resources for this Mattermost start. A view reads a picture once,
// and at most MAX_PICTURES of them, so a thread with many people is not as
// many requests each time. Where the host reads none, or hands back no
// picture, a person is their initials and an emoji its name.

const MAX_PICTURES = 60;
const PICTURE_TIMEOUT_MS = 20000;
const RASTER = new Set(["image/png", "image/jpeg", "image/gif", "image/webp"]);

const pictures = new Map();

// carriedPicture is the picture at uri that the result carried, for a host
// that passes on no read, as a data address, or null.
function carriedPicture(uri) {
  const carried = view.carried;
  if (!isObject(carried) || !Object.hasOwn(carried, uri)) return null;
  return typeof carried[uri] === "string" && EMBEDDED_IMAGE.test(carried[uri]) ? carried[uri] : null;
}

// readPicture is the picture at uri as a data address, or null.
function readPicture(uri) {
  const carried = carriedPicture(uri);
  if (carried) return Promise.resolve(carried);
  if (!view.readsResources || typeof uri !== "string" || uri === "") return Promise.resolve(null);
  if (pictures.has(uri)) return pictures.get(uri);
  if (pictures.size >= MAX_PICTURES) return Promise.resolve(null);
  const read = bridge.readResource(uri, PICTURE_TIMEOUT_MS).then((result) => {
    const contents = result && Array.isArray(result.contents) ? result.contents[0] : null;
    if (!contents || typeof contents.blob !== "string" || !RASTER.has(contents.mimeType)) return null;
    const url = "data:" + contents.mimeType + ";base64," + contents.blob;
    return EMBEDDED_IMAGE.test(url) ? url : null;
  }).catch(() => null);
  pictures.set(uri, read);
  return read;
}

function pictureOf(userID) {
  return readPicture(RESOURCE_BASE + "/users/" + encodeURIComponent(userID) + "/image");
}

// emoji is an emoji by its name: one that comes with Mattermost as its
// character, a custom one as its picture once that arrives, and the name
// itself until then, or when there is none.
function emoji(name) {
  const character = emojiCharacter(name);
  if (character) return el("span", { class: "emoji", role: "img", "aria-label": ":" + name + ":", title: ":" + name + ":" }, character);
  const box = el("span", { class: "emoji emoji-custom", title: ":" + name + ":" }, ":" + name + ":");
  readPicture(RESOURCE_BASE + "/emoji/" + encodeURIComponent(name)).then((url) => {
    if (!url) return;
    const img = el("img", { alt: ":" + name + ":", src: url });
    if (img.hasAttribute("src")) box.replaceChildren(img);
  });
  return box;
}

// emojiCharacter is the character of an emoji that comes with Mattermost, by
// any of its names, from SYSTEM_EMOJI, Mattermost's own table of them.
function emojiCharacter(name) {
  const key = String(name || "").toLowerCase();
  if (!Object.prototype.hasOwnProperty.call(SYSTEM_EMOJI, key)) return "";
  try {
    return SYSTEM_EMOJI[key].split("-").map((code) => String.fromCodePoint(parseInt(code, 16))).join("");
  } catch {
    return "";
  }
}
