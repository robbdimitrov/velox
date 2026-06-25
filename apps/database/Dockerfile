FROM postgres:16-alpine

COPY migrations/*.sql /docker-entrypoint-initdb.d/10-migrations.sql
COPY seeds/*.sql /docker-entrypoint-initdb.d/20-seeds.sql
