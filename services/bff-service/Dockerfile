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
ENV BFF_INVENTORY_GRPC_ADDR="inventory-service:50052"
ENV BFF_INVENTORY_GRPC_TIMEOUT="3s"
ENV BFF_CART_GRPC_ADDR="cart-service:50053"
ENV BFF_CART_GRPC_TIMEOUT="3s"
ENV BFF_ORDER_GRPC_ADDR="order-service:50055"
ENV BFF_ORDER_GRPC_TIMEOUT="15s"
ENV BFF_PAYMENT_GRPC_ADDR="payment-service:50056"
ENV BFF_PAYMENT_GRPC_TIMEOUT="5s"
ENV BFF_STOREFRONT_BASE_URL="/"
EXPOSE 8080

CMD ["/app/bff-server"]
