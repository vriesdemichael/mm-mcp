// Building the page.
//
// Every element is made with the DOM API, and text that came from Mattermost
// only ever becomes a text node: nothing on this page parses a string as HTML.
// A message is written by someone else, and in a view a script that got in
// could call mm-mcp's tools through the host.

const SVG_NS = "http://www.w3.org/2000/svg";

// An image the view shows is a picture mm-mcp read and the host passed on, as
// data. The view has no network, and a source that is anything else is
// dropped.
const EMBEDDED_IMAGE = /^data:image\/(png|jpeg|gif|webp);base64,[A-Za-z0-9+/]+=*$/;

function el(tag, props, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(props || {})) {
    if (value === undefined || value === null || value === false) continue;
    if (key === "class") {
      node.className = value;
    } else if (key === "text") {
      node.textContent = value;
    } else if (key === "dataset") {
      Object.assign(node.dataset, value);
    } else if (key.startsWith("on")) {
      if (typeof value !== "function") throw new Error("an event handler must be a function");
      node.addEventListener(key.slice(2), value);
    } else if (key === "src") {
      if (tag === "img") showImage(node, value);
    } else if (key === "href" || key === "srcdoc" || key === "style") {
      // Links open through the host, and nothing styles itself from data.
      throw new Error("el does not set " + key);
    } else {
      node.setAttribute(key, value === true ? "" : String(value));
    }
  }
  append(node, children);
  return node;
}

function append(node, children) {
  for (const child of children.flat(Infinity)) {
    if (child === undefined || child === null || child === false) continue;
    node.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return node;
}

// showImage gives an img its picture, when the picture is one the view may
// show.
function showImage(img, url) {
  if (typeof url === "string" && EMBEDDED_IMAGE.test(url)) img.setAttribute("src", url);
}

// Icons are drawn from fixed shapes on a 16-unit grid. A "path" is a stroke,
// a "disc" a fill.
const ICON_SHAPES = {
  file: [["path", "M4 1.5h5l3 3v10H4z"], ["path", "M9 1.5v3h3"]],
  image: [["path", "M2 3h12v10H2z"], ["path", "M2 11l3.5-3.5 3 3 2-2L14 12"], ["disc", 11, 6, 1.2]],
  pin: [["path", "M6 2h4l-.5 4 2.5 2.5H4L6.5 6z"], ["path", "M8 8.5V14"]],
  edited: [["path", "M10.5 2.5l3 3-8 8H2.5v-3z"]],
  open: [["path", "M9 2h5v5"], ["path", "M14 2L7.5 8.5"], ["path", "M12 9.5V14H2V4h4.5"]],
  more: [["path", "M4 6l4 4 4-4"]],
  check: [["path", "M3 8.5l3 3 7-7"]],
  flame: [["path", "M8 14.5a4 4 0 0 1-4-4c0-2.8 2.4-4.4 3-8c2 1.4 5 4.4 5 8a4 4 0 0 1-4 4z"], ["path", "M8 14.5a1.6 1.6 0 0 1-1.6-1.6c0-1.2 1-1.8 1.6-3.2c.6 1.4 1.6 2 1.6 3.2A1.6 1.6 0 0 1 8 14.5z"]],
  alert: [["path", "M8 2l6.5 11.5h-13z"], ["path", "M8 6.5v3"], ["disc", 8, 11.5, 0.7]],
  "alert-circle": [["path", "M8 1.5a6.5 6.5 0 1 0 0 13a6.5 6.5 0 1 0 0-13z"], ["path", "M8 4.5v4"], ["disc", 8, 11, 0.7]],
  phone: [["path", "M3.5 2h2.5l1.2 3-1.6 1.1a8 8 0 0 0 3.3 3.3l1.1-1.6 3 1.2v2.5a1.5 1.5 0 0 1-1.6 1.5A11.5 11.5 0 0 1 2 3.6A1.5 1.5 0 0 1 3.5 2z"]],
};

function icon(name, label) {
  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("viewBox", "0 0 16 16");
  svg.setAttribute("class", "icon icon-" + name);
  if (label) {
    svg.setAttribute("role", "img");
    svg.setAttribute("aria-label", label);
  } else {
    svg.setAttribute("aria-hidden", "true");
  }
  for (const shape of ICON_SHAPES[name] || []) {
    let part;
    if (shape[0] === "disc") {
      part = document.createElementNS(SVG_NS, "circle");
      part.setAttribute("cx", shape[1]);
      part.setAttribute("cy", shape[2]);
      part.setAttribute("r", shape[3]);
      part.setAttribute("class", "fill");
    } else {
      part = document.createElementNS(SVG_NS, "path");
      part.setAttribute("d", shape[1]);
      part.setAttribute("class", "stroke");
    }
    svg.append(part);
  }
  return svg;
}

// avatar is a person's picture, drawn as their initials until the picture
// arrives, and kept as initials when it does not.
function avatar(person) {
  const name = person.name || person.username || "";
  const box = el("span", { class: "avatar", role: "img", "aria-label": t("post.avatar", { name }) });
  box.dataset.hue = String(hueOf(person.id || name));
  box.append(el("span", { class: "initials", "aria-hidden": "true", text: initialsOf(name) }));
  if (person.id) {
    pictureOf(person.id).then((url) => {
      if (!url) return;
      const img = el("img", { alt: "", src: url });
      if (img.hasAttribute("src")) box.replaceChildren(img);
    });
  }
  return box;
}

// openLink asks the host to open a web address. A host that refuses gets the
// address shown, to copy.
function openLink(url) {
  if (!isWebURL(url)) return;
  if (!view.opensLinks) {
    notice(t("link.copy", { url }));
    return;
  }
  bridge.openLink(url).then((result) => {
    if (result && result.isError) notice(t("link.copy", { url }));
  }).catch(() => notice(t("link.copy", { url })));
}

// linkButton is a link as a button: the view has no anchors, since a page in a
// host's frame cannot follow one.
function linkButton(url, children, props) {
  if (!isWebURL(url)) return el("span", props || {}, children);
  return el("button", Object.assign({ type: "button", class: "link", title: url, onclick: () => openLink(url) }, props || {}), children);
}

function isWebURL(url) {
  try {
    const parsed = new URL(String(url || ""));
    return parsed.protocol === "https:" || parsed.protocol === "http:";
  } catch {
    return false;
  }
}
