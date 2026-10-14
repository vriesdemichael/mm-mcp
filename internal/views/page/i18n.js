// The page's own words, from a catalog per language. CATALOGS holds them all,
// English first; a word a language lacks is said in English. What people
// wrote in Mattermost is never translated.
//
// A word is a string with {placeholders}, or, when it counts something, its
// plural forms as Intl.PluralRules names them, each with {count}.

let language = "en";
let pluralRules = new Intl.PluralRules("en");

// setLanguage takes the first of the languages given that has a catalog, by
// its full tag or its language alone, and English otherwise.
function setLanguage(...tags) {
  for (const tag of tags) {
    if (typeof tag !== "string" || tag === "") continue;
    const found = catalogFor(tag);
    if (found) {
      language = found;
      break;
    }
  }
  pluralRules = new Intl.PluralRules(language);
  const root = document.documentElement;
  root.lang = language;
  root.dir = rightToLeft(language) ? "rtl" : "ltr";
}

function catalogFor(tag) {
  const exact = Object.keys(CATALOGS).find((name) => name.toLowerCase() === tag.toLowerCase());
  if (exact) return exact;
  const base = tag.split(/[-_]/)[0].toLowerCase();
  return Object.keys(CATALOGS).find((name) => name.toLowerCase() === base) || "";
}

function rightToLeft(tag) {
  return ["ar", "fa", "he", "ur", "yi", "ps", "sd", "ug", "dv", "ckb"].includes(tag.split(/[-_]/)[0].toLowerCase());
}

// t is the word key names in the page's language, with values put in its
// placeholders. A count picks the plural form.
function t(key, values) {
  const given = values || {};
  let word = wordOf(CATALOGS[language], key, given);
  if (word === null) word = wordOf(CATALOGS.en, key, given);
  if (word === null) return key;
  return word.replace(/\{([a-z_]+)\}/g, (whole, name) => {
    if (!Object.prototype.hasOwnProperty.call(given, name)) return whole;
    const value = given[name];
    return typeof value === "number" ? formatNumber(value) : String(value);
  });
}

function wordOf(catalog, key, values) {
  if (!catalog || !Object.prototype.hasOwnProperty.call(catalog, key)) return null;
  const word = catalog[key];
  if (typeof word === "string") return word;
  if (word && typeof word === "object") {
    const form = pluralRules.select(Number(values.count) || 0);
    if (typeof word[form] === "string") return word[form];
    if (typeof word.other === "string") return word.other;
  }
  return null;
}
