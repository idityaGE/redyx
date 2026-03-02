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
WORKDIR /app
EXPOSE 50051
ENTRYPOINT ["/bin/service"]
