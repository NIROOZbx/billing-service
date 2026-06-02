FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/backend ./cmd/

FROM alpine:3.23

WORKDIR /app

RUN adduser -D nirooz && chown -R nirooz:nirooz /app

USER nirooz

COPY --chown=nirooz:nirooz --from=builder /app/backend .

COPY --chown=nirooz:nirooz ./config.yaml ./config.yaml

EXPOSE 8081 50051

CMD ["./backend"]
