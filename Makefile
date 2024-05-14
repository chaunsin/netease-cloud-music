
test:
	go test -v ./..

build:
	go build -o ncm cmd/main.go

clean:
	rm -rf ./mcm