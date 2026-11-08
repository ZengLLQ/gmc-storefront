FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates tzdata
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM golang:1.23-alpine AS tools
RUN apk add --no-cache ca-certificates tzdata
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.22.1
COPY --from=builder /out/seed /bin/seed
COPY db/migrations /migrations
COPY db/seed /seed
WORKDIR /

FROM gcr.io/distroless/static-debian12:nonroot AS runner
COPY --from=builder /out/api /api
COPY --from=builder /src/db/seed /seed
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
CMD ["serve"]
