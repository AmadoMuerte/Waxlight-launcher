import { cn } from "@/shared/lib/utils";
import { CoverArt } from "@/shared/ui/cover-art";

export function ModArtwork({
  src,
  alt,
  seed,
  className = "",
}: {
  src?: string;
  alt: string;
  seed?: string;
  className?: string;
}) {
  return (
    <CoverArt src={src} alt={alt} seed={seed ?? alt} className={cn("aspect-[16/9]", className)} />
  );
}
