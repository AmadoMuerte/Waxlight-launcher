import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import { Copy, Download, Eraser, FolderOpen, Terminal as TerminalIcon } from "lucide-react";
import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { errorMessage } from "../../shared/api/bridge";
import { logsApi } from "../../shared/api/logs";
import { Button } from "../../shared/ui/button";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const consoleTheme = {
  background: "#0d0d10",
  foreground: "#eeeae4",
  cursor: "#e5a84b",
  selectionBackground: "#3b373c",
  black: "#0d0d10",
  red: "#e07168",
  green: "#7fb069",
  yellow: "#e5a84b",
  blue: "#7aa7c7",
  magenta: "#b28cbf",
  cyan: "#8fc7b9",
  white: "#eeeae4",
  brightBlack: "#999590",
  brightRed: "#e07168",
  brightGreen: "#7fb069",
  brightYellow: "#f1bd61",
  brightBlue: "#7aa7c7",
  brightMagenta: "#b28cbf",
  brightCyan: "#8fc7b9",
  brightWhite: "#ffffff",
};

export function LogConsole() {
  const { t } = useTranslation();
  const notify = useToastStore((state) => state.notify);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const linesRef = useRef<string[]>([]);
  const [exporting, setExporting] = useState(false);
  const [copied, setCopied] = useState(false);

  function pushLines(lines: string[]) {
    if (!lines.length) {
      return;
    }
    linesRef.current.push(...lines);
    terminalRef.current?.write(lines.join("\r\n") + "\r\n");
  }

  const copyText = useEffectEvent(async (text: string) => {
    if (!text) {
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      notify(t("logs_copy_failed"), "error");
    }
  });

  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return () => undefined;
    }
    const terminal = new Terminal({
      convertEol: true,
      disableStdin: true,
      cursorBlink: false,
      fontSize: 12,
      lineHeight: 1.25,
      fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", monospace',
      theme: consoleTheme,
      scrollback: 4000,
    });
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(container);
    fitAddon.fit();
    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;
    terminal.attachCustomKeyEventHandler((event) => {
      if (
        (event.ctrlKey || event.metaKey) &&
        event.key.toLowerCase() === "c" &&
        terminal.hasSelection()
      ) {
        void copyText(terminal.getSelection());
        return false;
      }
      return true;
    });

    const unsubscribe = EventsOn("logs:append", (line: unknown) => {
      pushLines([typeof line === "string" ? line : ""]);
    });

    void logsApi
      .list()
      .then((lines) => pushLines(lines ?? []))
      .catch(() => undefined);

    return () => {
      unsubscribe?.();
      terminalRef.current = null;
      fitAddonRef.current = null;
      terminal.dispose();
    };
  }, []);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return () => undefined;
    }
    const observer = new ResizeObserver(() => fitAddonRef.current?.fit());
    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  async function exportLogs() {
    setExporting(true);
    try {
      const path = await logsApi.exportLogs();
      if (path) {
        notify(t("logs_exported", { path }));
      }
    } catch (exportError) {
      notify(errorMessage(exportError), "error");
    } finally {
      setExporting(false);
    }
  }

  function copyAll() {
    return copyText(linesRef.current.join("\n"));
  }

  function clearConsole() {
    linesRef.current = [];
    terminalRef.current?.clear();
  }

  async function openLogsDirectory() {
    try {
      await logsApi.openDirectory();
    } catch (openError) {
      notify(errorMessage(openError), "error");
    }
  }

  return (
    <section className="logConsole">
      <header className="logConsoleHeader">
        <div className="logConsoleTitle">
          <TerminalIcon size={16} />
          <strong>{t("logs_console")}</strong>
        </div>
        <div className="row">
          <Button variant="ghost" aria-label={t("copy_logs")} onClick={() => void copyAll()}>
            <Copy size={15} />
            {copied ? t("logs_copied") : t("copy_logs")}
          </Button>
          <Button variant="ghost" aria-label={t("clear_logs")} onClick={clearConsole}>
            <Eraser size={15} />
            {t("clear_logs")}
          </Button>
          <Button
            variant="ghost"
            aria-label={t("open_logs_directory")}
            onClick={() => void openLogsDirectory()}
          >
            <FolderOpen size={15} />
            {t("open_logs_directory")}
          </Button>
          <Button variant="secondary" busy={exporting} onClick={() => void exportLogs()}>
            <Download size={15} />
            {exporting ? t("exporting_logs") : t("export_logs")}
          </Button>
        </div>
      </header>
      <div className="logConsoleBody" ref={containerRef} />
    </section>
  );
}
