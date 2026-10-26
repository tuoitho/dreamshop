FROM golang:1.24-alpine AS builder

ENV GOWORK=off
WORKDIR /app
COPY contracts/go ./contracts/go
COPY services/catalog-service/go.mod services/catalog-service/go.sum ./services/catalog-service/

WORKDIR /app/services/catalog-service
RUN go mod download

COPY services/catalog-service ./
RUN CGO_ENABLED=0 go build -o /catalog-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /catalog-server /app/catalog-server

ENV HTTP_ADDR=":8081"
ENV GRPC_ADDR="0.0.0.0:50051"
EXPOSE 8081 50051

CMD ["/app/catalog-server"]
