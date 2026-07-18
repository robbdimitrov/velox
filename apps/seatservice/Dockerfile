FROM rust:1.95-alpine3.21 AS builder
RUN apk add --no-cache build-base librdkafka-dev pkgconfig
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && printf 'pub fn placeholder() {}\n' > src/lib.rs && printf 'fn main() {}\n' > src/main.rs
RUN RUSTFLAGS="-C target-feature=-crt-static" cargo build --release --no-default-features --features kafka-dynamic
COPY src ./src
RUN find src -name '*.rs' -exec touch {} + && RUSTFLAGS="-C target-feature=-crt-static" cargo build --release --no-default-features --features kafka-dynamic
FROM alpine:3.21
RUN apk add --no-cache libgcc librdkafka
COPY --from=builder /src/target/release/velox-seatservice /seatservice
EXPOSE 8080
CMD ["/seatservice"]
