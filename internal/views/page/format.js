// Numbers, sizes and times, in the page's language.
//
// A time comes from mm-mcp in the person's own Mattermost timezone, with its
// offset, as 2026-10-09T09:30:00+02:00, and is shown as written there: the
// clock on the person's wall in Mattermost, not the host's.

function formatNumber(value) {
  return new Intl.NumberFormat(language).format(value);
}

function formatSize(bytes) {
  const units = [["GB", 1e9], ["MB", 1e6], ["KB", 1e3]];
  for (const [unit, size] of units) {
    if (bytes >= size) {
      return new Intl.NumberFormat(language, { maximumFractionDigits: 1 }).format(bytes / size) + " " + unit;
    }
  }
  return t("size.bytes", { count: bytes });
}

const WALL_TIME = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?/;

// wallTime is the time as written, the clock on the wall where it was
// written, as a Date to be read in UTC.
function wallTime(stamp) {
  const match = WALL_TIME.exec(String(stamp || ""));
  if (!match) return null;
  const [, year, month, day, hour, minute, second] = match;
  return new Date(Date.UTC(+year, +month - 1, +day, +hour, +minute, +(second || 0)));
}

function formatTime(stamp) {
  const at = wallTime(stamp);
  if (!at) return "";
  return new Intl.DateTimeFormat(language, { hour: "numeric", minute: "2-digit", timeZone: "UTC" }).format(at);
}

function formatDay(stamp) {
  const at = wallTime(stamp);
  if (!at) return "";
  return new Intl.DateTimeFormat(language, { weekday: "short", year: "numeric", month: "short", day: "numeric", timeZone: "UTC" }).format(at);
}

function formatMoment(stamp) {
  const at = wallTime(stamp);
  if (!at) return "";
  return new Intl.DateTimeFormat(language, { dateStyle: "full", timeStyle: "short", timeZone: "UTC" }).format(at);
}

function dayOf(stamp) {
  return String(stamp || "").slice(0, 10);
}

function initialsOf(name) {
  const words = String(name || "").replace(/^@/, "").split(/[\s._-]+/).filter(Boolean);
  const letters = words.length > 1 ? [words[0], words[words.length - 1]] : words.slice(0, 1);
  return letters.map((word) => Array.from(word)[0]).join("").toUpperCase() || "?";
}

// hueOf picks one of a few hues for a person, the same each time.
function hueOf(key) {
  let hash = 0;
  for (const ch of String(key || "")) hash = (hash * 31 + ch.codePointAt(0)) >>> 0;
  return hash % 8;
}
