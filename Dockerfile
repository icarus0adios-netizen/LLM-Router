
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o gateway cmd/gateway/main.go

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/gateway /usr/local/bin/gateway

COPY configs/gateway.yaml /etc/gateway/config.yaml

EXPOSE 8080

# 容器启动时执行的命令
ENTRYPOINT ["gateway", "/etc/gateway/config.yaml"]