FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ufshare .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /data

COPY --from=builder /app/ufshare /usr/local/bin/ufshare

EXPOSE 8000

ENTRYPOINT ["ufshare"]
CMD ["-host", "0.0.0.0", "-port", "8000", "-dir", "/data"]
