FROM golang:1.24-alpine AS builder

ENV GOWORK=off
WORKDIR /app
COPY contracts/go ./contracts/go
COPY services/inventory-service/go.mod services/inventory-service/go.sum ./services/inventory-service/

WORKDIR /app/services/inventory-service
RUN go mod download

COPY services/inventory-service ./
RUN CGO_ENABLED=0 go build -o /inventory-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /inventory-server /app/inventory-server

ENV HTTP_ADDR=":8082"
ENV GRPC_ADDR="0.0.0.0:50052"
ENV IDENTITY_GRPC_ADDR="identity-service:50054"
ENV IDENTITY_GRPC_TIMEOUT="500ms"
EXPOSE 8082 50052

CMD ["/app/inventory-server"]
