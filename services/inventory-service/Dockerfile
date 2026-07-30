FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.work ./
COPY contracts/go ./contracts/go
COPY services ./services

WORKDIR /app/services/inventory-service
RUN go build -o /inventory-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /inventory-server /app/inventory-server

ENV HTTP_ADDR=":8082"
EXPOSE 8082

CMD ["/app/inventory-server"]
