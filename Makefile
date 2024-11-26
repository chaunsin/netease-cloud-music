export IMAGE_VERSION ?= latest
export IMAGE_NAME?=chaunsin/ncmctl:${IMAGE_VERSION}
CURRENT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT_HASH := $(shell git rev-parse --short=7 HEAD)
BUILD_TIME=$(shell date "+%Y-%m-%d %H:%M:%S%z")

info:
	@echo "Current Branch: $(CURRENT_BRANCH)"
	@echo "Current Commit Hash: $(COMMIT_HASH)"
	@echo "Current Build Time: $(BUILD_TIME)"

test:
	#go test -v ./..

build: info
	go build -ldflags "-X 'main.Version=$(CURRENT_BRANCH)' -X 'main.Commit=${COMMIT_HASH}' -X 'main.BuildTime=${BUILD_TIME}' -s -w" -o ncmctl cmd/ncmctl/main.go

install:
	cd cmd/ncmctl && go install .

# 构建镜像
build-image:
	DOCKER_BUILDKIT=1 docker build --progress=plain -t $(IMAGE_NAME) -f $(PWD)/Dockerfile $(PWD)

# 推送镜像到doker hub
push-image:
	docker push $(IMAGE)

# 当使用docker部署时,如果没有登录账号则需要先登录
login:
	docker run --rm -it -v ./data:/root chaunsin/ncmctl:$(VERSION) /app/ncmctl login qrcode

# 运行服务，注意挂载的目录和登录挂载的目录要一致
task:
	docker run -it -d -v ./data:/root chaunsin/ncmctl:$(VERSION) /app/ncmctl task --sign --scrobble
	#docker-compose up -d