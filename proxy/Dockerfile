FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /mogra-proxy .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /mogra-proxy /mogra-proxy

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/mogra-proxy"]
