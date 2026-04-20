# Build a partir da raiz do monorepo (ex.: Render com Root Directory vazio).
# Para build só com pasta backend, use backend/Dockerfile e context = backend.
FROM golang:1.25-bookworm AS builder
WORKDIR /app
COPY backend/go.mod ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /texticulo .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /texticulo /texticulo
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/texticulo"]
