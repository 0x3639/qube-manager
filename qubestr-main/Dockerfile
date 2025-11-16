FROM golang:1.24.4-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o qubestr-relay ./cmd/qubestr

FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/qubestr-relay .
COPY --from=builder /app/.env.example .env

EXPOSE 3334

CMD ["./qubestr-relay"]
