FROM rust:alpine AS builder
RUN apk add --no-cache musl-dev gcc make cmake perl build-base curl-dev zlib-dev zlib-static openssl-dev
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY src ./src
RUN cargo build --release
FROM alpine:3.20
COPY --from=builder /src/target/release/velox-seatservice /seatservice
EXPOSE 8080
CMD ["/seatservice"]
