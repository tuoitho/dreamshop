FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.work ./
COPY contracts/go ./contracts/go
COPY services ./services

WORKDIR /app/services/catalog-service
RUN go build -o /catalog-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /catalog-server /app/catalog-server

ENV HTTP_ADDR=":8081"
EXPOSE 8081

CMD ["/app/catalog-server"]
