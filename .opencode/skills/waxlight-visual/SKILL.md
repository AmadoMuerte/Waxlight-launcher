---
name: waxlight-visual
description: Use when visually checking Waxlight Launcher pages, Wails frontend changes, layout, screenshots, browser console errors, or network failures.
---

# Waxlight Visual Check

Use the global headed `playwright` MCP. Do not write browser automation or add browser dependencies.

1. Start the application from `cmd/waxlight`:
   ```bash
   wails dev -browser -devserver localhost:34116
   ```
   Keep the process ID from the shell that started it. Stop only that process when finished.
2. Wait for `http://localhost:34116` before using Playwright. Port `34115` is the Vite server and has no Wails backend bridge; never open it directly.
3. With the Playwright MCP, inspect these routes through the hash router:
   - `http://localhost:34116/#/dev/ui`
   - `http://localhost:34116/#/servers`
4. For each relevant page, use a DOM/accessibility snapshot, inspect console and network errors, and take a screenshot when visual evidence is useful.
5. Navigate by visible controls first. Use a route URL only when checking a specific page directly.
6. Browser-mode checks frontend rendering. If the Wails bridge is unavailable in a plain browser, record it as an expected environment limitation, not a product regression. Use the native Wails window for bridge-specific behavior.
7. Report the route, viewport, observed layout result, and actionable console/network errors. Do not dump routine logs.
