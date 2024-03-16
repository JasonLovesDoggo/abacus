FROM golang:1.22@sha256:0b55ab82ac2a54a6f8f85ec8b943b9e470c39e32c109b766bbc1b801f3fa8d3b     as builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o ./abacus
# multistage build
FROM alpine:latest
COPY --from=builder /src/abacus /abacus
EXPOSE 8080
CMD ["/abacus"]

