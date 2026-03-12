FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -ldflags="-s -w" -o /config-service .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /config-service /config-service

EXPOSE 8083

ENTRYPOINT ["/config-service"]
