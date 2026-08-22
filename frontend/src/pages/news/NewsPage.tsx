import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Newspaper, RefreshCw } from "lucide-react";
import { memo, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import type { NewsItem } from "../../entities/news/model";
import { useNewsQuery } from "../../entities/news/queries";
import { errorMessage } from "../../shared/api/bridge";
import { NEWS_QUERY_KEY } from "../../shared/api/keys";
import { newsApi } from "../../shared/api/news";
import type { NewsFeed } from "../../shared/api/types";
import { formatCalendarDate } from "../../shared/lib";
import { log } from "../../shared/lib/logger";
import { Button } from "../../shared/ui/button";
import { Card } from "../../shared/ui/card";
import { EmptyState } from "../../shared/ui/empty";
import { ErrorState } from "../../shared/ui/error-state";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";

const NewsCard = memo(function NewsCard({ item }: { item: NewsItem }) {
  const { t } = useTranslation();
  const notify = useToastStore((state) => state.notify);
  const [imageFailed, setImageFailed] = useState(false);

  const openArticle = () => {
    void newsApi.openArticle(item.url).catch((error) => notify(errorMessage(error), "error"));
  };

  return (
    <Card className="overflow-hidden">
      <article
        className={
          item.imageUrl && !imageFailed ? "md:grid md:grid-cols-[minmax(240px,38%)_1fr]" : ""
        }
      >
        {item.imageUrl && !imageFailed && (
          <img
            className="aspect-video size-full max-h-72 object-cover md:aspect-auto"
            src={item.imageUrl}
            alt=""
            loading="lazy"
            onError={() => setImageFailed(true)}
          />
        )}
        <div className="flex min-w-0 flex-col gap-3 p-5 md:p-6">
          <div className="flex flex-wrap items-center gap-2 text-xs font-semibold text-text-muted">
            <span className="rounded-full bg-accent-muted px-2.5 py-1 text-accent-hover">
              {t(`news_category_${item.category}`)}
            </span>
            <time dateTime={item.publishedAt}>{formatCalendarDate(item.publishedAt)}</time>
          </div>
          <h2 className="font-display text-2xl font-semibold leading-tight text-text-primary">
            {item.title}
          </h2>
          {item.summary && (
            <p className="line-clamp-4 text-sm leading-6 text-text-muted">{item.summary}</p>
          )}
          <div className="mt-auto pt-1">
            <Button
              variant="ghost"
              className="px-0"
              aria-label={t("read_more_article", { title: item.title })}
              onClick={openArticle}
            >
              {t("read_more")} →
            </Button>
          </div>
        </div>
      </article>
    </Card>
  );
});

export function NewsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const query = useNewsQuery();
  const items = query.data?.items;

  useEffect(() => {
    if (!items?.length) {
      return;
    }
    const ids = items.map((item) => item.id);
    const marked = new Set(ids);
    void newsApi
      .markSeen(ids)
      .then(() =>
        queryClient.setQueryData<NewsFeed>(NEWS_QUERY_KEY, (current) =>
          current
            ? {
                ...current,
                unreadCount: current.items.every((item) => marked.has(item.id))
                  ? 0
                  : current.unreadCount,
              }
            : current,
        ),
      )
      .catch((error) => log.warn(errorMessage(error), { source: "news-mark-seen" }));
  }, [items, queryClient]);

  const refresh = useMutation({
    mutationFn: () => newsApi.sync(true),
    onSuccess: (feed) => queryClient.setQueryData(NEWS_QUERY_KEY, feed),
  });
  const refreshFailed =
    query.data?.refreshFailed || refresh.isError || (query.isError && query.data !== undefined);

  return (
    <Page>
      <PageHeader
        eyebrow={t("official_vintage_story")}
        title={t("news")}
        description={t("news_description")}
        actions={
          query.data && (
            <Button variant="secondary" busy={refresh.isPending} onClick={() => refresh.mutate()}>
              <span className="flex items-center gap-2">
                <RefreshCw size={15} aria-hidden="true" />
                {t("refresh")}
              </span>
            </Button>
          )
        }
      />

      <PageContent>
        {query.isPending && !query.data ? (
          <LoadingState>{t("loading_news")}</LoadingState>
        ) : query.isError && !query.data ? (
          <ErrorState
            title={t("unable_to_load_news")}
            description={t("news_unavailable_description")}
            action={<Button onClick={() => void query.refetch()}>{t("retry")}</Button>}
          />
        ) : !query.data || query.data.items.length === 0 ? (
          <EmptyState
            icon={<Newspaper size={24} aria-hidden="true" />}
            title={t("no_news")}
            description={t("no_news_description")}
          />
        ) : (
          <div className="flex flex-col gap-4">
            {refreshFailed && (
              <output className="block">
                <Card variant="subtle" className="px-4 py-3 text-sm text-text-muted">
                  <strong className="text-text-primary">{t("could_not_refresh_news")}</strong>{" "}
                  {t("showing_cached_news")}
                </Card>
              </output>
            )}
            {query.data.items.map((item) => (
              <NewsCard key={item.id} item={item} />
            ))}
          </div>
        )}
      </PageContent>
    </Page>
  );
}
