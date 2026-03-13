FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod ./
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /oms ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget tzdata

COPY --from=builder /oms /oms
RUN chmod +x /oms

EXPOSE 8084

CMD ["/oms"]
