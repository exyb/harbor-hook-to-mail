export CGO_ENABLED=0
export GOOS=linux
export GOARCH=arm64

go build -ldflags "-s -w" -o build/harbor-hook-to-mail.arm64
upx build/harbor-hook-to-mail.arm64

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

go build -ldflags "-s -w" -o build/harbor-hook-to-mail.amd64
upx build/harbor-hook-to-mail.amd64

