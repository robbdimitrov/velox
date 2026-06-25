FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/apigateway .

FROM scratch
COPY --from=builder /out/apigateway /apigateway
EXPOSE 8080
CMD ["/apigateway"]
