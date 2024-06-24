
test:
	go test -v ./..

build:
	go build -o ncmctl cmd/main.go

clean:
	rm -rf ./mcmctl