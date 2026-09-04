.PHONY: all daemon cli tools test lint cross check-desktop-deps desktop install

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

# The daemon and the CLI have to keep building for every platform the desktop
# client runs on. Compiling is not running, but it catches the mistake that
# actually happens: a Linux-only call reached for without a build tag.
cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dev/null ./...
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /dev/null ./...
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /dev/null ./...
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /dev/null ./...
	GOOS=windows GOARCH=amd64 go vet ./...
	GOOS=darwin GOARCH=arm64 go vet ./...

check-desktop-deps:
	@./scripts/check-desktop-deps.sh

desktop: check-desktop-deps tools
	cmake -S desktop -B desktop/build -DCMAKE_BUILD_TYPE=Release
	cmake --build desktop/build --parallel
	cmake -E copy_if_different bin/whatsappd desktop/build/whatsappd
	cmake -E copy_if_different bin/whatsappctl desktop/build/whatsappctl

install: desktop
	cmake --install desktop/build
