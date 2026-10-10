#!/bin/sh
# Создание ролей целевой БД с безопасным экранированием переменных psql.
set -eu

psql \
  -v ON_ERROR_STOP=1 \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" \
  --set=runtime_password="$DB_USER_PASSWORD" \
  --set=admin_password="$DB_ADMIN_PASSWORD" \
  --set=database="$POSTGRES_DB" <<'SQL'
CREATE ROLE "user"
    LOGIN PASSWORD :'runtime_password'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;

CREATE ROLE admin
    LOGIN PASSWORD :'admin_password'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;

REVOKE ALL ON DATABASE :"database" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"database" TO "user", admin;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
ALTER SCHEMA public OWNER TO admin;
GRANT USAGE ON SCHEMA public TO "user";

ALTER DEFAULT PRIVILEGES FOR ROLE admin IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO "user";

ALTER DEFAULT PRIVILEGES FOR ROLE admin IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO "user";

REVOKE CONNECT ON DATABASE postgres FROM PUBLIC;
REVOKE CONNECT ON DATABASE template1 FROM PUBLIC;
SQL
