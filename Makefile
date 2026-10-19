.PHONY: test cover

test:
	go test -race ./...

cover:
	go test -race -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML report: coverage.html"
