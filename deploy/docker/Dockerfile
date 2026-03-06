FROM golang:1.26-alpine AS builder
ARG SERVICE=skeleton
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd/${SERVICE}

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/service /bin/service
COPY --from=builder /app/migrations/ /app/migrations/
COPY --from=builder /app/internal/spam/data/ /app/internal/spam/data/
WORKDIR /app
EXPOSE 50051
ENTRYPOINT ["/bin/service"]
