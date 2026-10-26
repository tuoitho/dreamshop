FROM golang:1.24-alpine AS builder

ENV GOWORK=off
WORKDIR /app
COPY contracts/go ./contracts/go
COPY services/cart-service/go.mod services/cart-service/go.sum ./services/cart-service/

WORKDIR /app/services/cart-service
RUN go mod download

COPY services/cart-service ./
RUN CGO_ENABLED=0 go build -o /cart-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /cart-server /app/cart-server

ENV HTTP_ADDR=":8084"
ENV GRPC_ADDR="0.0.0.0:50053"
ENV CATALOG_GRPC_ADDR="catalog-service:50051"
ENV INVENTORY_GRPC_ADDR="inventory-service:50052"
ENV IDENTITY_GRPC_ADDR="identity-service:50054"
EXPOSE 8084 50053

CMD ["/app/cart-server"]
