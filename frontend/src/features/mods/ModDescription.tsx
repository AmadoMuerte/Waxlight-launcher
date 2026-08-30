import { createContext, useContext, useState, type ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";

const defaultAttributes = defaultSchema.attributes ?? {};

const modDescriptionSchema = {
  ...defaultSchema,
  tagNames: [...(defaultSchema.tagNames ?? []), "img"],
  attributes: {
    ...defaultAttributes,
    a: [...(defaultAttributes.a ?? []), "href", "title"],
    img: ["src", "alt", "title", "width", "height"],
  },
  protocols: {
    ...defaultSchema.protocols,
    href: ["https"],
    src: ["https"],
  },
};

function httpsURL(value?: string): string | undefined {
  if (!value) return undefined;
  try {
    const url = new URL(value);
    return url.protocol === "https:" ? url.href : undefined;
  } catch {
    return undefined;
  }
}

const OpenExternalContext = createContext<((url: string) => void) | null>(null);

function DescriptionLink({ href, children }: { href?: string; children?: ReactNode }) {
  const onOpenExternal = useContext(OpenExternalContext);
  const url = httpsURL(href);
  if (!url || !onOpenExternal) return <>{children}</>;

  return (
    <a
      href={url}
      onClick={(event) => {
        event.preventDefault();
        onOpenExternal(url);
      }}
    >
      {children}
    </a>
  );
}

function DescriptionImage({ src, alt }: { src?: string; alt?: string }) {
  const url = httpsURL(src);
  const [failedURL, setFailedURL] = useState<string>();
  if (!url || failedURL === url) return null;

  return (
    <img
      src={url}
      alt={alt ?? ""}
      loading="lazy"
      referrerPolicy="no-referrer"
      onError={() => setFailedURL(url)}
    />
  );
}

const descriptionComponents = { a: DescriptionLink, img: DescriptionImage };

export function ModDescription({
  description,
  fallback,
  onOpenExternal,
}: {
  description: string;
  fallback: string;
  onOpenExternal: (url: string) => void;
}) {
  if (!description.trim()) {
    return <p className="text-sm leading-7 whitespace-pre-wrap text-text-secondary">{fallback}</p>;
  }

  return (
    <OpenExternalContext value={onOpenExternal}>
      <div className="markdownBody text-text-secondary">
        <ReactMarkdown
          rehypePlugins={[rehypeRaw, [rehypeSanitize, modDescriptionSchema]]}
          urlTransform={(url) => httpsURL(url) ?? ""}
          components={descriptionComponents}
        >
          {description}
        </ReactMarkdown>
      </div>
    </OpenExternalContext>
  );
}
