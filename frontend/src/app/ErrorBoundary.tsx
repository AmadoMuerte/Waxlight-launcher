import { Component, type ErrorInfo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { errorMessage } from "../shared/api/bridge";
import { log } from "../shared/lib/logger";
import { Button } from "../shared/ui/button";
import { useAppShellStore } from "./stores/app-shell";

const MAX_STACK_LENGTH = 1000;

function truncate(value: string): string {
  return value.length > MAX_STACK_LENGTH ? `${value.slice(0, MAX_STACK_LENGTH)}…` : value;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

// ErrorBoundary catches render errors, logs them into the launcher console,
// and shows a recoverable fallback instead of a blank window.
export class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    useAppShellStore.getState().setFatalError(errorMessage(error));
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    log.error(error.message || "React render error", {
      stack: truncate(error.stack ?? ""),
      componentStack: truncate(info.componentStack ?? ""),
    });
  }

  render() {
    if (this.state.hasError) {
      return <BoundaryFallback />;
    }
    return this.props.children;
  }
}

function BoundaryFallback() {
  const { t } = useTranslation();
  const message = useAppShellStore((state) => state.fatalError);

  return (
    <div className="errorBoundary">
      <strong>{t("frontend_error")}</strong>
      <p>{message}</p>
      <Button variant="secondary" onClick={() => window.location.reload()}>
        {t("reload_launcher")}
      </Button>
    </div>
  );
}
