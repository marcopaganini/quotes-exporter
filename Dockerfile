FROM golang:1.13-alpine3.10 AS builder

MAINTAINER Marco Paganini <paganini@paganini.net>

ARG PROJECT="quotes-exporter"
ARG TOKEN="define-this-when-you-build"
ARG UID=60000

# Copy the repo contents into /tmp/build
WORKDIR /tmp/build
COPY . .

RUN export HOME=/tmp && \
    cd /tmp/build && \
    go mod download && \
    go build && \
    echo "${TOKEN}" >/tmp/token

# Build the small image
FROM alpine
WORKDIR /app
COPY --from=builder /tmp/build/quotes-exporter .
COPY --from=builder /tmp/token .

EXPOSE 9340
USER ${UID}
ENTRYPOINT [ "./quotes-exporter", "--tokenfile=/app/token" ]

