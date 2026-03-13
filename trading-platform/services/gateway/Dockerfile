FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
ARG TARGETARCH

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod ./
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /gateway ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /gateway /gateway
RUN chmod +x /gateway

EXPOSE 8080 8081

CMD ["/gateway"]
