FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /build
COPY . .

RUN go install github.com/rakyll/statik@latest
RUN go mod download
RUN cd webserver && statik -f -src=public
RUN go build -o /lwnsimulator cmd/main.go

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /lwnsimulator .
COPY --from=builder /build/config.json .

EXPOSE 8002 8003

CMD ["./lwnsimulator"]
