FROM golang:1-alpine AS builder

MAINTAINER Marco Paganini <paganini@paganini.net>

ARG PROJECT="quotes-exporter"
ARG UID=60000

# Copy the repo contents into /tmp/build
WORKDIR /tmp/build
COPY . .

RUN export HOME=/tmp && \
    cd /tmp/build && \
    go mod download && \
    go build

# Build the small image
FROM alpine
WORKDIR /app
COPY --from=builder /tmp/build/quotes-exporter .

EXPOSE 9340
USER ${UID}
ENTRYPOINT [ "./quotes-exporter" ]

