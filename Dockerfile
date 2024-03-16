FROM golang:1.22@sha256:0b55ab82ac2a54a6f8f85ec8b943b9e470c39e32c109b766bbc1b801f3fa8d3b     as builder
WORKDIR /go/src/abacus
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -v -o main
# multistage build
FROM alpine:latest
COPY --from=builder /go/src/abacus/abacus /abacus
EXPOSE 8080
CMD ["/abacus"]

