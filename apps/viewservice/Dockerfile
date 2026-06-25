FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/viewservice .

FROM scratch
COPY --from=builder /out/viewservice /viewservice
EXPOSE 8080
CMD ["/viewservice"]
