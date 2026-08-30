import { useTranslation } from "react-i18next";

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
  if (screenshots.length === 0) return null;

  function label(item: Screenshot, itemIndex: number) {
    return item.caption || t("screenshot_alt", { name: modName, number: itemIndex + 1 });
  }

  return (
    <section aria-label={t("screenshots")} className="flex gap-2 overflow-x-auto pb-1">
      {screenshots.map((item, itemIndex) => {
        const itemLabel = label(item, itemIndex);
        return (
          <button
            key={item.url}
            type="button"
            aria-label={`${itemLabel} (${t("open")})`}
            onClick={() => onOpen(itemIndex)}
            className="h-16 w-28 shrink-0 overflow-hidden rounded-md border border-border-subtle transition-colors hover:border-border-strong focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent sm:h-20 sm:w-36"
          >
            <img src={item.url} alt="" className="h-full w-full object-cover" />
          </button>
        );
      })}
    </section>
  );
}
