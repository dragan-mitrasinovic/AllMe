# Build the Angular app, then serve it with nginx, which also proxies /api/* to the backend.
FROM node:24.22.3-alpine AS build

WORKDIR /app

# Accept build configuration as argument (defaults to production)
ARG BUILD_CONFIGURATION=production

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build -- --configuration ${BUILD_CONFIGURATION}

# Unprivileged nginx (runs as a non-root user, listens on 8080)
FROM nginxinc/nginx-unprivileged:1.30-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist/frontend/browser /usr/share/nginx/html

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
