# Сборка и раздача клиентского приложения через локальный HTTP-прокси.
FROM node:22.22.0-alpine AS build

WORKDIR /app

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM nginx:1.28.2-alpine

COPY infra/proxy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
