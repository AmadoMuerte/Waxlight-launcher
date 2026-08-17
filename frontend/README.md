# Waxlight Frontend — Design System

This document is the practical contract for the Waxlight UI. It exists so the
design system survives contact with new features. Read it before writing UI.

## Design direction

Dark charcoal surfaces, warm amber accent, desktop-first, minimal, quiet.
Waxlight is a launcher, not a gaming RGB dashboard. Every screen should read
as part of one product.

## Rules

### Use semantic tokens, not raw values

All colors, radii, shadows, and durations come from design tokens in
`src/app/styles/tokens.css`. Never write `#hex`, `rgb(...)`, or arbitrary
`text-[#…]`/`bg-[#…]` classes in shared UI.

Raw values are allowed only for genuinely domain-specific cases:

- external provider branding (e.g. the Discord mark);
- data visualization palettes (e.g. the xterm console theme in
  `LogConsole.tsx`);
- deliberate hero/artwork treatments (e.g. the instance hero mark).

### Use shared primitives

Everything in `src/shared/ui/` is canonical. Do not hand-roll a button,
input, dialog, dropdown, or status pill on a page.

- `Button` — variants `primary | secondary | ghost | danger`. Primary for the
  one main action in a context; `danger` for destructive actions; cancel uses
  `ghost` or `secondary`.
- `IconButton` — the only way to render a standalone clickable icon. Always
  pass an `aria-label`.
- `StatusPill` — the only way to render a status. Do not color text by status
  by hand.
- `Dialog` / `Modal` / `ConfirmDialog` — all dialogs. Use `DialogFooter` for
  footer actions, order: `Cancel  Primary/Danger`.
- `EmptyState` / `ErrorState` / `LoadingState` — the only empty/error/loading
  presentations. Use full-page error states only for page-level failures.
- `Field`, `Input`, `Select`, `Switch`, `Checkbox`, `Stepper`,
  `SegmentedControl`, `Tabs`, `Toolbar`, `PageHeader`, `SectionHeader`,
  `SettingRow`, `Card` — the building blocks for forms, toolbars, and cards.

### Icons

Use Lucide only. One concept, one icon. Common mappings:

```
Delete → Trash2      Edit → Pencil      Folder → FolderOpen
Refresh → RefreshCw  Copy → Copy        External link → ExternalLink
More → MoreHorizontal  Play → Play      Search → Search
```

No unicode symbols as icons, no inline SVGs unless it is a custom mark
(brand, artwork).

### Buttons in forms

`Button` defaults to `type="button"`. The submit action of a `SubmitForm`
must set `type="submit"` explicitly.

### Layout

- Pages use `Page` / `PageContent` / `PageSection` with `PageHeader` and
  `SectionHeader`.
- Toolbars use `Toolbar` / `ToolbarGroup`.
- Cards: `Card` / `CardHeader` / `CardTitle` / `CardContent` / `CardFooter`.
  Domain cards (Instance, Mod, Server, Account) share the visual language but
  stay domain-specific — do not build one universal card.

### Status vs badge vs metadata

- Status = lifecycle state of an entity (`running`, `installed`, `failed`) →
  `StatusPill`.
- Badge = a small labeled tag (channel, source, latest mark) → custom badge
  classes in `styles/domain.css`.
- Metadata = supporting text → `text-text-muted`.

### Data flow

Server data goes through TanStack Query (entity hooks + query keys in
`shared/api/keys.ts`); refresh with `queryClient.invalidateQueries`. Client-only
state goes through zustand. Never prop-drill data or refresh callbacks into
pages.

## Structure

```
src/app/styles/
├── tokens.css      design tokens (colors, radii, shadows, spacing, motion)
├── base.css        global layout + shared primitives
└── domain.css      page/feature-specific styling
src/app/styles.css  imports tokens/base/domain + responsive + reduced-motion
```

Shared UI lives in `src/shared/ui/` as one file per component. Import
directly from the file, never from a barrel.

## UI Lab

`/dev/ui` (route `dev-ui` → `UiLabPage`) showcases the real production
components with mock data. It exists to catch regressions in shared/domain
components. It is development-only: the route and nav entry are stripped from
the production bundle via `import.meta.env.DEV`. When adding a shared or
domain component, add a showcase to the UI Lab.

## Validation

- `npm run build` runs i18n check, lint, format check, `tsc`, and the Vite
  production build.
- `npm test` runs the vitest suite.
- `make test` from the repository root runs the full stack (frontend build +
  tests, Go tests).

Run `npm run build` and `npm test` before committing. The pre-commit hook runs
`make format-check lint`.
