FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
ARG TARGETARCH

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod ./
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /watchdog .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /watchdog /watchdog
RUN chmod +x /watchdog

EXPOSE 8082

CMD ["/watchdog"]
