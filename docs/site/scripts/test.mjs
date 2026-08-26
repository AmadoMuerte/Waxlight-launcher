import { access, readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const api = JSON.parse(await readFile(new URL("../../generated/wails-api.json", import.meta.url)));
const config = await readFile(new URL("../.vitepress/config.mts", import.meta.url), "utf8");

if (!config.includes("api.controllers.map") || !config.includes("api.types.map")) {
  throw new Error("VitePress sidebar must derive controllers and types from the API schema");
}

for (const controller of api.controllers) {
  await access(new URL(`api/controllers/${controller.name}.md`, root));
}
for (const type of api.types) {
  await access(new URL(`api/types/${type.name}.md`, root));
}
await access(new URL("api/README.md", root));
await access(new URL("api/METHODS.md", root));
await access(new URL("getting-started.md", root));
await access(new URL("long-running-operations.md", root));

for (const route of [
  "api/README.html",
  "api/METHODS.html",
  "getting-started.html",
  "long-running-operations.html",
]) {
  await access(new URL(`.vitepress/dist/${route}`, root));
}

console.log(`verified ${api.controllers.length} controllers and ${api.types.length} types`);
