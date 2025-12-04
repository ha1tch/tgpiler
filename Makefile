.PHONY: build test clean examples install

# Build the CLI
build:
	go build -o tgpiler ./cmd/tgpiler

# Install globally
install:
	go install ./cmd/tgpiler

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f tgpiler
	rm -f examples/*.go

# Transpile all examples
examples: build
	@mkdir -p examples/go
	@for f in examples/*.sql; do \
		name=$$(basename "$$f" .sql); \
		echo "Transpiling $$f -> examples/go/$$name.go"; \
		./tgpiler -p examples -o "examples/go/$$name.go" -f "$$f"; \
	done

# Verify examples compile
verify: examples
	@echo "Verifying generated Go code compiles..."
	@cd examples/go && go build -o /dev/null ./...
	@echo "All examples compile successfully"

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Lint
lint:
	go vet ./...
