# syntax=docker/dockerfile:1

FROM ghcr.io/gohugoio/hugo:v0.163.3 AS builder

WORKDIR /site
COPY . .
RUN hugo --minify

FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY --from=builder /site/public/ /usr/share/nginx/html/
COPY nginx/rate-limit.conf /etc/nginx/conf.d/rate-limit.conf
COPY nginx/default.conf /etc/nginx/conf.d/default.conf

USER 101

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/ >/dev/null || exit 1
