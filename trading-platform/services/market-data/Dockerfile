FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod ./
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /market-data ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget tzdata

COPY --from=builder /market-data /market-data
RUN chmod +x /market-data

EXPOSE 8083

CMD ["/market-data"]
