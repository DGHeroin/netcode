NAME = netcode
GO_BUILD = go build
RELEASE_DIR ?= release

$(shell mkdir -p ${RELEASE_DIR})

#all: linux-amd64 linux-arm64 darwin-amd64 windows-amd64
all: linux-amd64  darwin-amd64 windows-amd64

linux-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=x86_64-linux-musl-gcc $(GO_BUILD) -o ${RELEASE_DIR}/$(NAME)-linux-amd64 -v

linux-arm64:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=arm-linux-musleabi-gcc $(GO_BUILD) -o ${RELEASE_DIR}/$(NAME)-linux-arm64 -v

darwin-amd64:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CC=clang $(GO_BUILD) -o ${RELEASE_DIR}/$(NAME)-darwin-amd64 -v

windows-amd64:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc $(GO_BUILD) -o ${RELEASE_DIR}/$(NAME)-windows-amd64.exe -v