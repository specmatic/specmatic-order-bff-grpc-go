FROM golang:1.25-alpine AS builder
RUN apk add --no-cache make git
WORKDIR /app
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0 && \
    go install github.com/bufbuild/buf/cmd/buf@v1.28.1
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make all

FROM alpine:3.23
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/specmatic-order-bff-grpc-go .
EXPOSE 8080

# Run the binary
CMD ["./specmatic-order-bff-grpc-go"]
