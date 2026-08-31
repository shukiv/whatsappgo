.PHONY: all daemon cli tools test lint check-desktop-deps desktop install

all: tools

daemon:
	CGO_ENABLED=0 go build -trimpath -o bin/whatsappd ./cmd/whatsappd

cli:
	CGO_ENABLED=0 go build -trimpath -o bin/whatsappctl ./cmd/whatsappctl

tools: daemon cli

test:
	go test ./...

lint:
	go vet ./...

check-desktop-deps:
	@./scripts/check-desktop-deps.sh

desktop: check-desktop-deps tools
	cmake -S desktop -B desktop/build -DCMAKE_BUILD_TYPE=Release
	cmake --build desktop/build --parallel
	cmake -E copy_if_different bin/whatsappd desktop/build/whatsappd
	cmake -E copy_if_different bin/whatsappctl desktop/build/whatsappctl

install: desktop
	cmake --install desktop/build
