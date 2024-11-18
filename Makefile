export IMAGE_VERSION ?= latest
export IMAGE_NAME?=chaunsin/ncmctl:${IMAGE_VERSION}

test:
	#go test -v ./..

build:
	go build -o ncmctl cmd/ncmctl/main.go

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