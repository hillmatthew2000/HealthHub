# syntax=docker/dockerfile:1
#
# HealthHub web app: a static Vue build served by nginx.
#
#   docker build -t healthhub-web web
#   docker run -p 8082:80 -e API_BASE_URL=http://192.168.1.96:8081 healthhub-web
#
# API_BASE_URL is read at container start (see docker/40-healthhub-config.sh),
# so the same image works in every environment.

FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
RUN npm run build

FROM nginx:alpine
COPY docker/nginx.conf /etc/nginx/nginx.conf
COPY --chmod=755 docker/40-healthhub-config.sh /docker-entrypoint.d/40-healthhub-config.sh
COPY --from=build /app/dist /usr/share/nginx/html
# The default site is replaced by nginx.conf. The build's placeholder
# config.js is replaced by the one written at start-up.
RUN rm -f /etc/nginx/conf.d/default.conf /usr/share/nginx/html/config.js

# Run as the image's unprivileged nginx user. Docker lets containers bind
# port 80 without root (net.ipv4.ip_unprivileged_port_start=0 by default).
USER nginx
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1/healthz || exit 1
