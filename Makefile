export PATH := $(GOPATH)/bin:$(PATH)

LDFLAGS := -s -w -X main.VERSION=$(VERSION) -X 'main.BuildTime=`date`' -X 'main.GoVersion=`go version`'

all: build

build: app

app:
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o ./out/proxy_pool_darwin_amd64
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o ./out/proxy_pool_linux_amd64
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o ./out/proxy_pool_linux_arm64
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o ./out/proxy_pool_windows_amd64.exe
