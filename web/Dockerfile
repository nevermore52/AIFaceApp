FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor ./vendor

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o server cmd/server/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations

EXPOSE 3000

CMD ["./server"]
