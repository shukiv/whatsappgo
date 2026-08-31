.PHONY: all daemon test lint check-desktop-deps desktop install

all: daemon

daemon:
	CGO_ENABLED=0 go build -trimpath -o bin/whatsappd ./cmd/whatsappd

test:
	go test ./...

lint:
	go vet ./...

check-desktop-deps:
	@./scripts/check-desktop-deps.sh

desktop: check-desktop-deps daemon
	cmake -S desktop -B desktop/build -DCMAKE_BUILD_TYPE=Release
	cmake --build desktop/build --parallel
	cmake -E copy_if_different bin/whatsappd desktop/build/whatsappd

install: desktop
	cmake --install desktop/build
