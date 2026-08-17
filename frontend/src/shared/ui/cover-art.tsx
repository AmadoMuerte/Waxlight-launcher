import type { CSSProperties } from "react";

import { cn } from "@/shared/lib/utils";

/* [gradient from, gradient to, monogram ink] — deep muted jewel duotones */
const PALETTES = [
  ["#3d2c16", "#17110a", "#eab464"],
  ["#24382b", "#0f1712", "#a3c9a8"],
  ["#2a3644", "#10151c", "#9ab8d6"],
  ["#3a2540", "#150f1a", "#c3a0d6"],
  ["#402226", "#170f11", "#d59a92"],
  ["#1f3d3a", "#0d1615", "#8fc7bc"],
] as const;

function paletteIndex(text: string): number {
  let hash = 0;
  for (let index = 0; index < text.length; index++) {
    hash = (hash * 31 + text.charCodeAt(index)) | 0;
  }
  return Math.abs(hash) % PALETTES.length;
}

export function CoverArt({
  src,
  alt = "",
  seed,
  className,
}: {
  src?: string;
  alt?: string;
  seed?: string;
  className?: string;
}) {
  const key = seed ?? alt;
  const [from, to, ink] = PALETTES[paletteIndex(key)];
  const letter =
    key
      .trim()
      .match(/[\p{L}\p{N}]/u)?.[0]
      ?.toUpperCase() ?? "W";
  const style: CSSProperties & Record<"--cover-from" | "--cover-to" | "--cover-ink", string> = {
    "--cover-from": from,
    "--cover-to": to,
    "--cover-ink": ink,
  };
  return (
    <div
      className={cn("coverArt", className)}
      style={style}
      {...(alt ? { role: "img", "aria-label": alt } : { "aria-hidden": true })}
    >
      <span className="coverArtMonogram" aria-hidden="true">
        {letter}
      </span>
      {src && (
        <img
          key={src}
          src={src}
          alt=""
          loading="lazy"
          onError={(event) => {
            event.currentTarget.hidden = true;
          }}
        />
      )}
    </div>
  );
}
