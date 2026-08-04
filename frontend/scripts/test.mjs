import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const frontendDirectory = path.resolve(scriptDirectory, "..");
const vitest = path.join(frontendDirectory, "node_modules", "vitest", "vitest.mjs");
const result = spawnSync(process.execPath, [vitest, "run"], {
  cwd: frontendDirectory,
  env: { ...process.env, NODE_ENV: "test" },
  stdio: "inherit",
});

process.exit(result.status ?? 1);
