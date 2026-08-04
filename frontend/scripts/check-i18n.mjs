import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const directory = fileURLToPath(new URL("../src/i18n/locales/", import.meta.url));
const config = JSON.parse(await readFile(new URL("../../languages.json", import.meta.url), "utf8"));
const files = (await readdir(directory)).filter((file) => file.endsWith(".json")).sort();
const load = async (file) => JSON.parse(await readFile(path.join(directory, file), "utf8"));
const codes = config.languages.map((language) => language.code);
const expectedFiles = new Set(codes.map((code) => `${code}.json`));
const resources = new Map(await Promise.all(files.map(async (file) => [file, await load(file)])));
const source = resources.get(`${config.defaultLanguage}.json`);
const sourceKeys = Object.keys(source)
  .filter((key) => key !== "_glossary")
  .sort();
let failed = false;

for (const file of expectedFiles) {
  if (!files.includes(file)) {
    console.error(`missing locale file "${file}"`);
    failed = true;
  }
}
for (const file of files) {
  if (!expectedFiles.has(file)) {
    console.error(`locale file "${file}" has no language code in languages.json`);
    failed = true;
    continue;
  }
  const resource = resources.get(file);
  const keys = Object.keys(resource)
    .filter((key) => key !== "_glossary")
    .sort();
  const missing = sourceKeys.filter((key) => !keys.includes(key));
  const extra = keys.filter((key) => !sourceKeys.includes(key));
  for (const key of missing) console.error(`${file}: missing key "${key}"`);
  for (const key of extra) console.error(`${file}: extra key "${key}"`);
  for (const key of keys) {
    if (typeof resource[key] !== "string") console.error(`${file}: "${key}" must be a string`);
    else if (resource[key].trim() === "") console.error(`${file}: "${key}" must not be empty`);
  }
  failed ||=
    missing.length > 0 ||
    extra.length > 0 ||
    keys.some((key) => typeof resource[key] !== "string" || resource[key].trim() === "");
}

if (failed) process.exitCode = 1;
else console.log(`i18n: ${codes.length} languages, ${sourceKeys.length} matching application keys`);
