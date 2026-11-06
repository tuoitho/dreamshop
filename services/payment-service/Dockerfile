FROM golang:1.24-alpine AS builder

ENV GOWORK=off
WORKDIR /app
COPY contracts/go ./contracts/go
COPY services/payment-service/go.mod services/payment-service/go.sum ./services/payment-service/

WORKDIR /app/services/payment-service
RUN go mod download

COPY services/payment-service ./
RUN CGO_ENABLED=0 go build -o /payment-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /payment-server /app/payment-server

ENV HTTP_ADDR=":8086"
ENV GRPC_ADDR="0.0.0.0:50056"
EXPOSE 8086 50056

CMD ["/app/payment-server"]
