
.PHONY: linter

linter:
	docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:v1.45.0 golangci-lint run -v