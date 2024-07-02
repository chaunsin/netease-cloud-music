
test:
	go test -v ./..

build:
	go build -o ncmctl cmd/ncmctl/main.go

install:
	cd cmd/ncmctl && go install .

#uninstall:
#	rm -rf ./mcmctl