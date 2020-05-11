.PHONY: image force-image build

bin := quotes-exporter
src := $(wildcard *.go)

# Default target
${bin}: Makefile ${src}
	go build -v -o "${bin}"

# Docker targets
image:
	docker build -t ${USER}/quotes-exporter .

force-image:
	docker build --no-cache -t ${USER}/quotes-exporter .
