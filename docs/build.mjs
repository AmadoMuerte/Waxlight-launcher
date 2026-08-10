/**
 * Waxlight Wiki generator.
 *
 * Scans content/<lang>/**.md, renders each file to a static HTML page and
 * writes assets/wiki-data.js (navigation, search, prev/next metadata).
 *
 * Usage: node wiki/build.mjs   (from frontend/) or npm run wiki:build
 *
 * Adding a page: drop a .md file into content/<lang>/<section>/ and rebuild.
 * No router, sidebar, or registry edits are required.
 */
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, posix } from "node:path";
import { fileURLToPath } from "node:url";
import React from "react";
import ReactDOMServer from "react-dom/server";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

const ROOT = dirname(fileURLToPath(import.meta.url));
const CONTENT = join(ROOT, "content");
const LANGS = ["ru", "en"];
const DEFAULT_SECTION_ORDER = 50;

/* Localized bits of generated page chrome (hero, community banner). */
const STRINGS = {
  ru: {
    heroLead:
      "Полная документация по Waxlight — независимому открытому лаунчеру для Vintage Story. Аккаунты, версии игры, изолированные инстансы, моды, бэкапы и статистика — в одном приложении для Windows и Linux.",
    badges: ["GPL-3.0", "Windows x64", "Linux x64", "10 языков интерфейса", "Open Source"],
    btnStart: "Начать работу",
    btnDownload: "Скачать с GitHub",
    btnDiscord: "Discord-сервер",
    bannerTitle: "Сообщество в Discord",
    bannerText:
      "У Waxlight есть официальный Discord-сервер: помощь с установкой, обсуждение функций, новости разработки и обратная связь. Заходите:",
    bannerJoin: "Вступить",
  },
  en: {
    heroLead:
      "The complete documentation for Waxlight — an independent open-source launcher for Vintage Story. Accounts, game versions, isolated instances, mods, backups, and playtime in one app for Windows and Linux.",
    badges: ["GPL-3.0", "Windows x64", "Linux x64", "10 interface languages", "Open Source"],
    btnStart: "Get started",
    btnDownload: "Download on GitHub",
    btnDiscord: "Discord server",
    bannerTitle: "Community on Discord",
    bannerText:
      "Waxlight has an official Discord server: install help, feature discussions, development news, and feedback. Join us:",
    bannerJoin: "Join",
  },
};

const LINKS = {
  discord: "https://discord.gg/CrRHvg9UVw",
  releases: "https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest",
};

const DISCORD_SVG =
  '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20.32 4.37a19.8 19.8 0 0 0-4.93-1.51 13.8 13.8 0 0 0-.64 1.28 18.3 18.3 0 0 0-5.5 0 13.8 13.8 0 0 0-.64-1.28c-1.71.29-3.37.8-4.93 1.51A20.3 20.3 0 0 0 .1 18.06a19.9 19.9 0 0 0 6.07 3.03c.49-.66.93-1.37 1.3-2.1a12.9 12.9 0 0 1-2.05-.98c.17-.12.34-.25.5-.38a14.2 14.2 0 0 0 12.16 0c.16.13.33.26.5.38-.65.39-1.34.72-2.05.98.37.73.81 1.44 1.3 2.1a19.9 19.9 0 0 0 6.07-3.03 20.3 20.3 0 0 0-3.58-13.69ZM8.02 15.33c-1.18 0-2.16-1.08-2.16-2.42s.95-2.42 2.16-2.42 2.18 1.09 2.16 2.42c0 1.34-.95 2.42-2.16 2.42Zm7.96 0c-1.18 0-2.16-1.08-2.16-2.42s.95-2.42 2.16-2.42 2.18 1.09 2.16 2.42c0 1.34-.95 2.42-2.16 2.42Z"/></svg>';

function humanize(name) {
  return name
    .split("-")
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}

function parseFrontmatter(src) {
  const m = src.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/);
  if (!m) return { data: {}, body: src };
  const data = {};
  for (const line of m[1].split(/\r?\n/)) {
    const kv = line.match(/^([A-Za-z_]+):\s*(.+?)\s*$/);
    if (kv) data[kv[1]] = kv[2].replace(/^["']|["']$/g, "");
  }
  if (data.order !== undefined) {
    const n = Number(data.order);
    data.order = Number.isFinite(n) ? n : undefined;
  }
  return { data, body: src.slice(m[0].length) };
}

function renderMarkdown(md) {
  return ReactDOMServer.renderToStaticMarkup(
    React.createElement(ReactMarkdown, { remarkPlugins: [remarkGfm] }, md),
  );
}

/* Relative link from one slug to another ("features/mods" from "index" -> "features/mods.html",
 * from "features/accounts" -> "mods.html"). */
function hrefBetween(fromSlug, toSlug) {
  const fromDir = posix.dirname(fromSlug);
  const rel = posix.relative(fromDir === "." ? "" : fromDir, toSlug + ".html");
  return rel.startsWith(".") ? rel : "./" + rel;
}

function sectionCards(langData, sectionId, fromSlug) {
  const section = langData.sections.find((s) => s.id === sectionId);
  if (!section) return `<p><!-- no such section: ${sectionId} --></p>`;
  const cards = section.pages
    .map(
      (p) =>
        `<a class="card" href="${hrefBetween(fromSlug, p.slug)}"><h3>${p.title}</h3><p>${p.description}</p></a>`,
    )
    .join("");
  return `<div class="card-grid">${cards}</div>`;
}

function discordBanner(lang) {
  const t = STRINGS[lang];
  return (
    `<div class="community-banner">${DISCORD_SVG}` +
    `<div><h2>${t.bannerTitle}</h2><p>${t.bannerText} <strong>discord.gg/CrRHvg9UVw</strong></p></div>` +
    `<a class="btn discord" href="${LINKS.discord}" target="_blank" rel="noopener">${t.bannerJoin}</a></div>`
  );
}

function hero(lang) {
  const t = STRINGS[lang];
  const badges = t.badges.map((b) => `<span>${b}</span>`).join("");
  return (
    `<section class="hero"><h1>Waxlight <span>Wiki</span></h1>` +
    `<p class="lead">${t.heroLead}</p>` +
    `<div class="hero-badges">${badges}</div>` +
    `<div class="actions">` +
    `<a class="btn primary" href="getting-started.html">${t.btnStart}</a>` +
    `<a class="btn" href="${LINKS.releases}" target="_blank" rel="noopener">${t.btnDownload}</a>` +
    `<a class="btn discord" href="${LINKS.discord}" target="_blank" rel="noopener">${t.btnDiscord}</a>` +
    `</div></section>`
  );
}

const ALERT_CLASS = { NOTE: "", WARNING: " warn", TIP: " ok" };

function postProcess(html, lang, fromSlug, langData) {
  let out = html;
  /* Placeholders (must sit on their own line in Markdown). */
  out = out.replace(/<p>\{\{hero\}\}<\/p>/, () => hero(lang));
  out = out.replace(/<p>\{\{discord-banner\}\}<\/p>/, () => discordBanner(lang));
  out = out.replace(/<p>\{\{cards:([a-z0-9-]+)\}\}<\/p>/g, (_, id) =>
    sectionCards(langData, id, fromSlug),
  );
  /* GitHub-style alerts -> callout blocks. */
  out = out.replace(
    /<blockquote>\s*<p>\[!(NOTE|WARNING|TIP)\][ \t]*([^\n<]*)\n?([\s\S]*?)<\/p>\s*<\/blockquote>/g,
    (_, kind, title, body) =>
      `<div class="callout${ALERT_CLASS[kind]}"><span class="callout-title">${title}</span>` +
      (body.trim() ? `<p>${body}</p>` : "") +
      `</div>`,
  );
  /* Internal Markdown links: ./page.md, ../section/page.md -> .html */
  out = out.replace(/href="([^"]+?)\.md(#[^"]*)?"/g, 'href="$1.html$2"');
  return out;
}

function htmlPage({ lang, slug, title, description, contentHtml }) {
  const depth = slug.includes("/") ? "../../" : "../";
  const desc = description ? `\n  <meta name="description" content="${description}">` : "";
  return `<!doctype html>
<!-- GENERATED from content/${lang}/${slug}.md by wiki/build.mjs — do not edit by hand -->
<html lang="${lang}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">${desc}
  <title>${title} — Waxlight Wiki</title>
  <link rel="icon" href="${depth}assets/waxlight.png">
  <link rel="stylesheet" href="${depth}assets/styles.css">
</head>
<body data-page="${slug}">
  <main class="wiki-main">
    <div class="wiki-content">
${contentHtml}
    </div>
  </main>
  <script src="${depth}assets/wiki-data.js"></script>
  <script src="${depth}assets/app.js"></script>
</body>
</html>
`;
}

function scanLang(lang) {
  const langDir = join(CONTENT, lang);
  const rootPages = [];
  const sections = [];

  function readPage(dir, sectionId) {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isFile() || !entry.name.endsWith(".md")) continue;
      const name = entry.name.replace(/\.md$/, "");
      const raw = readFileSync(join(dir, entry.name), "utf8");
      const { data, body } = parseFrontmatter(raw);
      const h1 = body.match(/^#\s+(.+)$/m);
      const slug = sectionId ? `${sectionId}/${name}` : name;
      const page = {
        slug,
        title: data.title || (h1 ? h1[1].trim() : humanize(name)),
        description: data.description || "",
        order: data.order,
        section: sectionId,
        body,
      };
      (sectionId ? sections.find((s) => s.id === sectionId).pages : rootPages).push(page);
    }
  }

  readPage(langDir, null);
  for (const entry of readdirSync(langDir, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const metaPath = join(langDir, entry.name, "_meta.json");
    const meta = existsSync(metaPath) ? JSON.parse(readFileSync(metaPath, "utf8")) : {};
    sections.push({
      id: entry.name,
      title: meta.title || humanize(entry.name),
      order: meta.order ?? DEFAULT_SECTION_ORDER,
      pages: [],
    });
    readPage(join(langDir, entry.name), entry.name);
  }

  const byOrderTitle = (a, b) =>
    (a.order ?? Number.POSITIVE_INFINITY) - (b.order ?? Number.POSITIVE_INFINITY) ||
    a.title.localeCompare(b.title, lang);
  sections.sort((a, b) => a.order - b.order || a.id.localeCompare(b.id));
  for (const s of sections) s.pages.sort(byOrderTitle);
  rootPages.sort(byOrderTitle);
  return { rootPages, sections };
}

function buildLang(lang) {
  const langData = scanLang(lang);
  const langOut = join(ROOT, lang);

  /* Remove previously generated pages (keep nothing hand-written under <lang>/). */
  if (existsSync(langOut)) rmSync(langOut, { recursive: true });

  const allPages = [...langData.rootPages, ...langData.sections.flatMap((s) => s.pages)];
  for (const page of allPages) {
    const rendered = postProcess(renderMarkdown(page.body), lang, page.slug, langData);
    const outPath = join(langOut, ...page.slug.split("/")) + ".html";
    mkdirSync(dirname(outPath), { recursive: true });
    writeFileSync(
      outPath,
      htmlPage({
        lang,
        slug: page.slug,
        title: page.title,
        description: page.description,
        contentHtml: rendered,
      }),
    );
  }

  return {
    pages: allPages.map((p) => ({
      slug: p.slug,
      title: p.title,
      description: p.description,
      section: p.section,
      path: p.slug + ".html",
    })),
    sections: langData.sections.map((s) => ({ id: s.id, title: s.title })),
  };
}

const data = { langs: {} };
for (const lang of LANGS) {
  data.langs[lang] = buildLang(lang);
  const count = data.langs[lang].pages.length;
  console.log(`wiki: built ${count} pages for ${lang}`);
}

const header = "/* GENERATED by wiki/build.mjs — do not edit by hand */\n";
writeFileSync(
  join(ROOT, "assets", "wiki-data.js"),
  header + "window.WIKI_DATA = " + JSON.stringify(data, null, 2) + ";\n",
);
console.log("wiki: wrote assets/wiki-data.js");
