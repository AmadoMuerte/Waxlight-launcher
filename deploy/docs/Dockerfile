FROM nginx:1.29.1-alpine

COPY deploy/docs/nginx.conf /etc/nginx/conf.d/default.conf
COPY docs/site/.vitepress/dist/ /usr/share/nginx/html/

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1/health || exit 1
