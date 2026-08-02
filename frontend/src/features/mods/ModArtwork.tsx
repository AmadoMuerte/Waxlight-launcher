import { useState } from "react";

export function ModArtwork({
  src,
  alt,
  className = "",
}: {
  src?: string;
  alt: string;
  className?: string;
}) {
  const [failed, setFailed] = useState(false);
  if (!src || failed) {
    return (
      <div className={`modArtwork modArtworkFallback ${className}`} aria-label={alt}>
        <span>W</span>
        <small>MODS</small>
      </div>
    );
  }
  return (
    <div className={`modArtwork ${className}`}>
      <img
        src={src}
        alt={alt}
        loading="lazy"
        onError={() => setFailed(true)}
      />
    </div>
  );
}
