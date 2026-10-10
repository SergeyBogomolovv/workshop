# Копирование скрипта в образ позволяет избежать ограничений запуска файлов с хоста.
FROM postgres:18.3-alpine

COPY --chmod=0644 init.sh /docker-entrypoint-initdb.d/01-roles.sh
