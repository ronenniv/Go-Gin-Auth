.PHONY: linter

linter:
	docker run -t --rm -v ${PWD}:/app -w /app golangci/golangci-lint:v1.50.1 golangci-lint run -v