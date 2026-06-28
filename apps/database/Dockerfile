FROM postgres:16-alpine

COPY migrations/ /docker-entrypoint-initdb.d/
COPY seeds/ /docker-entrypoint-initdb.d/
