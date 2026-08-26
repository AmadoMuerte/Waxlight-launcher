import { readFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import { defineConfig, type DefaultTheme } from "vitepress";

type API = {
  controllers: Array<{ name: string; methods: unknown[] }>;
  types: Array<{ name: string }>;
};

const api = JSON.parse(
  readFileSync(fileURLToPath(new URL("../../generated/wails-api.json", import.meta.url)), "utf8"),
) as API;

const title = "Waxlight Backend API";

const controllers = api.controllers.map(({ name }) => ({
  text: name,
  link: `/api/controllers/${name}`,
}));
const types = api.types.map(({ name }) => ({ text: name, link: `/api/types/${name}` }));
const methodCount = api.controllers.reduce((count, controller) => count + controller.methods.length, 0);

const sidebar: DefaultTheme.SidebarItem[] = [
  { text: "Getting Started", link: "/getting-started" },
  { text: "Long-running Operations", link: "/long-running-operations" },
  { text: "Overview", link: "/api/README" },
  { text: "Methods", link: "/api/METHODS" },
  { text: "Controllers", collapsed: false, items: controllers },
  { text: "Types", collapsed: true, items: types },
];

export default defineConfig({
  title,
  description: "Generated documentation for Waxlight's public Wails backend API.",
  cleanUrls: true,
  head: [["meta", { name: "theme-color", content: "#6d55d7" }]],
  transformPageData(pageData) {
    if (pageData.relativePath === "index.md") {
      pageData.frontmatter.apiCounts = {
        controllers: api.controllers.length,
        methods: methodCount,
        types: api.types.length,
      };
    }
  },
  themeConfig: {
    nav: [
      { text: "Home", link: "/" },
      { text: "Getting Started", link: "/getting-started" },
      { text: "API Reference", link: "/api/README" },
      { text: "Methods", link: "/api/METHODS" },
    ],
    sidebar: { "/api/": sidebar },
    search: { provider: "local" },
    socialLinks: [
      { icon: "github", link: "https://github.com/AmadoMuerte/Waxlight-launcher" },
    ],
  },
});
