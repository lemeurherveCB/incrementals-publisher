FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bom-results-publisher .

FROM scratch
COPY --from=builder /bom-results-publisher /bom-results-publisher
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENV PORT=3000
EXPOSE 3000
ENTRYPOINT ["/bom-results-publisher"]
