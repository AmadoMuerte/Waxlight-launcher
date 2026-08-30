import { ChevronLeft, ChevronRight } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Card } from "../../shared/ui/card";
import { IconButton } from "../../shared/ui/icon-button";

type Screenshot = { url: string; caption?: string };

export function ModScreenshotCarousel({
  screenshots,
  modName,
  onOpen,
}: {
  screenshots: Screenshot[];
  modName: string;
  onOpen: (index: number) => void;
}) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState(0);
  const index = Math.min(selected, Math.max(screenshots.length - 1, 0));
  const screenshot = screenshots[index];

  if (!screenshot) return null;

  function label(item: Screenshot, itemIndex: number) {
    return item.caption || t("screenshot_alt", { name: modName, number: itemIndex + 1 });
  }

  function move(amount: number) {
    setSelected((current) => (current + amount + screenshots.length) % screenshots.length);
  }

  const currentLabel = label(screenshot, index);

  return (
    <Card className="overflow-hidden">
      <div className="relative bg-surface-input">
        <button
          type="button"
          className="block aspect-video w-full cursor-zoom-in overflow-hidden focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-4px] focus-visible:outline-accent"
          onClick={() => onOpen(index)}
        >
          <img src={screenshot.url} alt={currentLabel} className="h-full w-full object-contain" />
          <span className="sr-only">{t("open")}</span>
        </button>

        <div className="pointer-events-none absolute inset-x-0 top-0 flex items-center justify-between p-4">
          <span className="rounded-full bg-bg-app/80 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-text-primary backdrop-blur">
            {t("screenshots")}
          </span>
          <span className="rounded-full bg-bg-app/80 px-3 py-1 text-xs font-medium text-text-secondary backdrop-blur">
            {index + 1} / {screenshots.length}
          </span>
        </div>

        {screenshots.length > 1 && (
          <>
            <IconButton
              className="absolute left-3 top-1/2 -translate-y-1/2 border border-border-subtle bg-bg-app/90 shadow-lg backdrop-blur hover:bg-surface-3"
              variant="ghost"
              aria-label={t("previous_screenshot")}
              onClick={() => move(-1)}
            >
              <ChevronLeft size={20} aria-hidden="true" />
            </IconButton>
            <IconButton
              className="absolute right-3 top-1/2 -translate-y-1/2 border border-border-subtle bg-bg-app/90 shadow-lg backdrop-blur hover:bg-surface-3"
              variant="ghost"
              aria-label={t("next_screenshot")}
              onClick={() => move(1)}
            >
              <ChevronRight size={20} aria-hidden="true" />
            </IconButton>
          </>
        )}
      </div>

      <div className="flex gap-2 overflow-x-auto border-t border-border-subtle p-3">
        {screenshots.map((item, itemIndex) => {
          const itemLabel = label(item, itemIndex);
          const active = itemIndex === index;
          return (
            <button
              key={item.url}
              type="button"
              aria-label={itemLabel}
              aria-pressed={active}
              onClick={() => setSelected(itemIndex)}
              className={`h-14 w-20 shrink-0 overflow-hidden rounded-md border transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent ${
                active
                  ? "border-accent ring-1 ring-accent"
                  : "border-border-subtle opacity-65 hover:border-border-strong hover:opacity-100"
              }`}
            >
              <img src={item.url} alt="" className="h-full w-full object-cover" />
            </button>
          );
        })}
      </div>

      {screenshot.caption && (
        <p className="border-t border-border-subtle px-4 py-3 text-sm text-text-secondary">
          {screenshot.caption}
        </p>
      )}
    </Card>
  );
}
