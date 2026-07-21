FROM rust:1.95-alpine3.21 AS builder
RUN apk add --no-cache build-base cmake curl-dev pkgconfig zlib-static
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && printf 'pub fn placeholder() {}\n' > src/lib.rs && printf 'fn main() {}\n' > src/main.rs
RUN cargo build --release
COPY src ./src
RUN find src -name '*.rs' -exec touch {} + && cargo build --release
FROM alpine:3.21
RUN apk add --no-cache ca-certificates libgcc
COPY --from=builder /src/target/release/velox-seatservice /seatservice
EXPOSE 8080
CMD ["/seatservice"]
