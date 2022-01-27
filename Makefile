.PHONY: test install
all: test install

app=$(notdir $(shell pwd))
appVersion := 1.0.0
goVersion := $(shell go version | sed 's/go version //'|sed 's/ /_/')
# e.g. 2021-10-28T11:49:52+0800
buildTime := $(shell date +%FT%T%z)
# https://git-scm.com/docs/git-rev-list#Documentation/git-rev-list.txt-emaIem
gitCommit := $(shell [ -f git.commit ] && cat git.commit || git rev-list --oneline --format=format:'%h@%aI' --max-count=1 `git rev-parse HEAD` | tail -1)
#gitCommit := $(shell git rev-list -1 HEAD)
# https://stackoverflow.com/a/47510909
pkg := github.com/bingoohuang/gg/pkg/v

extldflags := -extldflags -static
# https://ms2008.github.io/2018/10/08/golang-build-version/
# https://github.com/kubermatic/kubeone/blob/master/Makefile
flags1 = -s -w -X $(pkg).BuildTime=$(buildTime) -X $(pkg).AppVersion=$(appVersion) -X $(pkg).GitCommit=$(gitCommit) -X $(pkg).GoVersion=$(goVersion)
flags2 = ${extldflags} ${flags1}
goinstall1 = go install -trimpath -ldflags='${flags1}' ./...
goinstall2 = go install -trimpath -ldflags='${flags2}' ./...
gobin := $(shell go env GOBIN)
# try $GOPATN/bin if $gobin is empty
gobin := $(if $(gobin),$(gobin),$(shell go env GOPATH)/bin)

git.commit:
	echo ${gitCommit} > git.commit

tool:
	go get github.com/securego/gosec/cmd/gosec
	go install github.com/golang/protobuf/protoc-gen-go@latest

sec:
	@gosec ./...
	@echo "[OK] Go security check was completed!"

init:
	export GOPROXY=https://goproxy.cn
	protoc --go_out=plugins=grpc:. proto/*.proto

lint-all:
	golangci-lint run --enable-all

lint:
	golangci-lint run ./...

fmt:
	gofumpt -l -w .
	gofmt -s -w .
	go mod tidy
	go fmt ./...
	revive .
	goimports -w .
	gci -w -local github.com/daixiang0/gci

install: init
	${goinstall1}
	upx ${gobin}/${app}
linux: init
	GOOS=linux GOARCH=amd64 ${goinstall1}
	upx ${gobin}/linux_amd64/${app}
arm: init
	GOOS=linux GOARCH=arm64 ${goinstall1}
	upx ${gobin}/linux_arm64/${app}

upx:
	ls -lh ${gobin}/${app}
	upx ${gobin}/${app}
	ls -lh ${gobin}/${app}
	ls -lh ${gobin}/linux_amd64/${app}
	upx ${gobin}/linux_amd64/${app}
	ls -lh ${gobin}/linux_amd64/${app}

test: init
	#go test -v ./...
	go test -v -race ./...

bench: init
	#go test -bench . ./...
	go test -tags bench -benchmem -bench . ./...

clean:
	rm coverage.out

cover:
	go test -v -race -coverpkg=./... -coverprofile=coverage.out ./...

coverview:
	go tool cover -html=coverage.out

# https://hub.docker.com/_/golang
# docker run --rm -v "$PWD":/usr/src/myapp -v "$HOME/dockergo":/go -w /usr/src/myapp golang make docker
# docker run --rm -it -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang bash
# 静态连接 glibc
docker:
	mkdir -p ~/dockergo
	docker run --rm -v "$$PWD":/usr/src/myapp -v "$$HOME/dockergo":/go -w /usr/src/myapp golang make dockerinstall
	#upx ~/dockergo/bin/${app}
	gzip -f ~/dockergo/bin/${app}

dockerinstall:
	go install -v -x -a -ldflags=${flags} ./...

targz:
	find . -name ".DS_Store" -delete
	find . -type f -name '\.*' -print
	cd .. && rm -f ${app}.tar.gz && tar czvf ${app}.tar.gz --exclude .git --exclude .idea ${app}

