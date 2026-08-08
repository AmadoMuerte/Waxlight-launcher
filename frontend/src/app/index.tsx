import { QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import ReactDOM from "react-dom/client";
import { HashRouter } from "react-router";

import "@xterm/xterm/css/xterm.css";
import "../shared/i18n";
import { setMissingKeyReporter } from "../shared/i18n";
import { installGlobalErrorLogging, log } from "../shared/lib/logger";
import { App } from "./App";
import { ErrorBoundary } from "./ErrorBoundary";
import { queryClient } from "./providers/queryClient";

import "./styles.css";

setMissingKeyReporter((languages, namespace, key) => {
  log.warn(`Missing translation: ${namespace}:${key}`, { languages: languages.join(",") });
});

installGlobalErrorLogging();

const rootElement = document.getElementById("root");

if (!rootElement) {
  throw new Error("The application root element was not found.");
}

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <HashRouter>
        <ErrorBoundary>
          <App />
        </ErrorBoundary>
      </HashRouter>
    </QueryClientProvider>
  </React.StrictMode>,
);
