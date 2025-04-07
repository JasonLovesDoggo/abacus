# Build stage
FROM golang:1.24 as builder
WORKDIR /src
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o ./abacus -tags=jsoniter

# Run stage
FROM alpine:latest
COPY --from=builder /src/abacus /abacus
COPY assets /assets
EXPOSE 8080
ENV GIN_MODE=release
#USER nonroot:nonroot
CMD ["/abacus"]

# note: curl is not installed by default in alpine so we use wget
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD wget -S -O - http://0.0.0.0:8080/healthcheck || exit 1

LABEL maintainer="Jason Cameron abacus@jasoncameron.dev"
LABEL version="1.5.3"
LABEL description="This is a simple countAPI service written in Go."
