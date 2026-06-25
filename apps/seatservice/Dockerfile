FROM rust:1.78-alpine AS builder
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY src ./src
RUN cargo build --release

FROM alpine:3.20
COPY --from=builder /src/target/release/velox-seatservice /seatservice
EXPOSE 8080
CMD ["/seatservice"]
