ARG GO_VERSION=1
FROM golang:${GO_VERSION}-alpine as builder

# Build stage
WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /run-app .

# Final stage
FROM alpine:latest

# Install ca-certificates on Alpine
RUN apk add --no-cache ca-certificates

# Copy the built application
COPY --from=builder /run-app /usr/local/bin/

# Ensure CA certificates are up to date
RUN update-ca-certificates

# Set the entrypoint
CMD ["run-app"]
