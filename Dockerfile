FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /kubesentinel ./cmd

FROM scratch

COPY --from=builder /kubesentinel .
COPY --from=builder /app/config /config

ENTRYPOINT ["/kubesentinel"]
