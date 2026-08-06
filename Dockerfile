FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /incrementals-publisher ./cmd/incrementals-publisher

FROM gcr.io/distroless/static-debian12:nonroot

ENV PORT 3000
EXPOSE 3000

COPY --from=builder /incrementals-publisher /incrementals-publisher

ENTRYPOINT ["/incrementals-publisher"]
