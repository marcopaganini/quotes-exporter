.PHONY: image force-image build

bin := quotes-exporter
src := $(wildcard *.go)

# Default target
${bin}: Makefile ${src}
	go build -v -o "${bin}"

# Docker targets
image: check-token
	docker build -t ${USER}/quotes-exporter --build-arg TOKEN=${TOKEN} .

force-image: check-token
	docker build --no-cache -t ${USER}/quotes-exporter --build-arg TOKEN=${TOKEN} .

check-token:
ifndef TOKEN
	$(error TOKEN is undefined)
endif
