import { readdir, readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const directory = fileURLToPath(new URL("../src/i18n/locales/", import.meta.url));
const files = (await readdir(directory)).filter((file) => file.endsWith(".json")).sort();
const load = async (file) => JSON.parse(await readFile(path.join(directory, file), "utf8"));
const source = await load("en.json");
const sourceKeys = Object.keys(source).filter((key) => key !== "_glossary").sort();
let failed = false;

for (const file of files) {
  const resource = await load(file);
  const keys = Object.keys(resource).filter((key) => key !== "_glossary").sort();
  const missing = sourceKeys.filter((key) => !keys.includes(key));
  const extra = keys.filter((key) => !sourceKeys.includes(key));
  for (const key of missing) console.error(`${file}: missing key "${key}"`);
  for (const key of extra) console.error(`${file}: extra key "${key}"`);
  for (const key of keys) {
    if (typeof resource[key] !== "string") console.error(`${file}: "${key}" must be a string`);
    else if (resource[key].trim() === "") console.error(`${file}: "${key}" must not be empty`);
  }
  failed ||= missing.length > 0 || extra.length > 0 || keys.some((key) => typeof resource[key] !== "string" || resource[key].trim() === "");
}

if (failed) process.exitCode = 1;
else console.log(`i18n: ${files.length} languages, ${sourceKeys.length} matching application keys`);
