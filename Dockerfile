FROM golang:1.25-alpine AS builder

# Install necessary tools including make
RUN apk add --no-cache make

# Install necessary tools
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/bufbuild/buf/cmd/buf@latest

# Set up the working directory
WORKDIR /app

# Copy go mod and sum files
COPY . .

# Download all dependencies
RUN go mod tidy
RUN go mod download

# # Copy the source code and Makefile
# COPY cmd ./cmd
# COPY internal ./internal
# COPY pkg ./pkg
# RUN ls 
# COPY Makefile ./

# Build the application
RUN make all

# # Final stage
# FROM golang:1.22-alpine

# RUN apk --no-cache add ca-certificates

# WORKDIR /root/

# # Copy the binary from the builder stage
# COPY --from=builder /app/specmatic-order-bff-grpc-go .

# Expose the port your service runs on
EXPOSE 8080

# Run the binary
CMD ["./specmatic-order-bff-grpc-go"]