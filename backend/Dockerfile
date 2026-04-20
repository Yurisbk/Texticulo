# Build
FROM golang:1.25-bookworm AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /texticulo .
# base-debian12 inclui CA certs para TLS (MongoDB Atlas)
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /
COPY --from=builder /texticulo /texticulo
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/texticulo"]
