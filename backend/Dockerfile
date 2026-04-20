# Build
FROM golang:1.25-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /texticulo .

# Run (distroless or alpine — static binary works on distroless)
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /texticulo /texticulo
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/texticulo"]
