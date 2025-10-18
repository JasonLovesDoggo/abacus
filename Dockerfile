
# Build stage
FROM golang:1.24 AS builder
WORKDIR /src
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o ./abacus -tags=jsoniter

# Run stage

FROM scratch

LABEL maintainer="Jason Cameron abacus@jasoncameron.dev"
LABEL version="1.5.5"
LABEL description="This is a simple countAPI service written in Go."


COPY --from=builder /src/abacus /abacus
COPY assets /assets
EXPOSE 8080
ENV GIN_MODE=release
CMD ["/abacus"]

