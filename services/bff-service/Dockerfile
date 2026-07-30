FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.work ./
COPY contracts/go ./contracts/go
COPY services ./services

WORKDIR /app/services/bff-service
RUN go build -o /bff-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /bff-server /app/bff-server

ENV HTTP_ADDR=":8080"
EXPOSE 8080

CMD ["/app/bff-server"]
