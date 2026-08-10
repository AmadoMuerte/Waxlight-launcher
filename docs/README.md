# Waxlight Wiki

Standalone bilingual (ru/en) documentation site. Plain HTML/CSS/JS output, no framework, no runtime build step.

## Adding a Wiki page

1. Create a Markdown file:

   ```text
   content/<lang>/<section>/<page>.md
   ```

   Example: `content/ru/troubleshooting/mod-download-errors.md`. A new directory automatically becomes a navigation section.

2. Add optional frontmatter:

   ```md
   ---
   title: Mod Download Errors
   description: Short summary used in cards and search.
   order: 20
   ---
   ```

   All fields are optional. `title` falls back to the first `# Heading`, then to a humanized file name. `order` controls sorting (pages without it sort alphabetically after ordered ones).

3. Write Markdown. Supported: headings, paragraphs, links, lists, inline code, code blocks, blockquotes, tables (GFM), images, bold/italic.

4. Rebuild and commit both the Markdown and the generated output:

   ```bash
   npm run wiki:build --prefix frontend
   ```

No router, sidebar, registry, or component edits are required — pages and sections are discovered from the file tree.

## Conventions

- **Mirror languages**: every page exists under both `content/ru/` and `content/en/` with the same relative path.
- **Internal links** between Wiki pages point at the Markdown source and are rewritten at build time: `[Backups](./backups.md)`, `[Security](../policies/security.md)`.
- **Callouts** use GitHub alert syntax: `> [!NOTE] Title`, `> [!WARNING] Title`, `> [!TIP] Title` (one body paragraph per callout).
- **Section labels/order**: optional `content/<lang>/<section>/_meta.json` with `{ "title": "…", "order": 30 }`. Without it the directory name is humanized.
- **Home page placeholders**: `{{hero}}`, `{{cards:features}}`, `{{cards:policies}}`, `{{discord-banner}}` (each on its own line).

## Layout

- `content/` — Markdown sources (the only files you edit for content changes).
- `assets/app.js` — shared chrome (header, sidebar, TOC, prev/next, search); reads `assets/wiki-data.js`.
- `assets/wiki-data.js` — generated navigation/search metadata. Do not edit by hand.
- `<lang>/**.html` — generated pages. Do not edit by hand.
- `build.mjs` — the generator. Run via `npm run wiki:build --prefix frontend`.
- `index.html` — root language redirect (hand-written).
