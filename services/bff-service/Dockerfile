FROM golang:1.24-alpine AS builder

ENV GOWORK=off
WORKDIR /app
COPY contracts/go ./contracts/go
COPY services/bff-service/go.mod services/bff-service/go.sum ./services/bff-service/

WORKDIR /app/services/bff-service
RUN go mod download

COPY services/bff-service ./
RUN CGO_ENABLED=0 go build -o /bff-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /bff-server /app/bff-server

ENV BFF_HTTP_ADDR=":8080"
ENV BFF_CATALOG_GRPC_ADDR="catalog-service:50051"
ENV BFF_CATALOG_GRPC_TIMEOUT="3s"
ENV BFF_CART_GRPC_ADDR="cart-service:50053"
ENV BFF_CART_GRPC_TIMEOUT="3s"
EXPOSE 8080

CMD ["/app/bff-server"]
