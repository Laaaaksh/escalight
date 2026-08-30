.PHONY: build run test lint tidy clean icons demo

BINARY := escalight
MODULE := github.com/Laaaaksh/escalight

build:
	go build -o $(BINARY) .

run:
	go run . serve

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
	go clean -testcache

icons:
	go run scripts/genicons.go

demo:
	cd scripts/record-demo && npm install && npx playwright install chromium && npm run record
