test:
	go test -v --race ./...

run:
	go run main.go

build:
	go build -o bin/go-scheduler main.go

lint:
	golangci-lint run --fix