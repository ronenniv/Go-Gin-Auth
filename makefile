.PHONY: linter

linter:
	docker run -t --rm -v ${PWD}:/app -w /app golangci/golangci-lint golangci-lint run -v