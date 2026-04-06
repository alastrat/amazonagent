FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./apps/api/main.go

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl

COPY --from=builder /bin/api /bin/api
COPY --from=builder /app/internal/adapter/postgres/migrations /migrations

EXPOSE 8080

CMD ["/bin/api"]
