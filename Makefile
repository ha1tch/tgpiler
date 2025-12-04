.PHONY: build test test-all test-quick test-compilation test-financial test-e2e test-e2e-compile test-e2e-execute clean install fmt lint

# Build the CLI
build:
	go build -o tgpiler ./cmd/tgpiler

# Install globally
install:
	go install ./cmd/tgpiler

# Run all tests (default)
test: build
	go test -v ./tests/...

# Alias for test
test-all: test

# Quick smoke test for fast feedback
test-quick: build
	go test -v ./tests/... -run "TestFactorial|TestGcd|TestLevenshtein|TestFutureValue|TestLoanPayment" -count=1

# Verify all SQL files transpile to valid Go syntax (gofmt)
test-compilation: build
	go test -v ./tests/... -run TestCompilation

# End-to-end: transpile + compile + execute
test-e2e: build
	go test -v ./tests/... -run TestE2E -count=1

# E2E compile only: transpile all 55 files and verify go build succeeds
test-e2e-compile: build
	go test -v ./tests/... -run TestE2ECompileAll -count=1

# E2E execute: transpile, compile, and run selected functions
test-e2e-execute: build
	go test -v ./tests/... -run "TestE2EExecuteBasic|TestE2EExecuteFinancial|TestE2EExecuteNontrivial" -count=1

# Run financial tests only
test-financial: build
	go test -v ./tests/... -run "TestCompilationFinancial|TestFuture|TestLoan|TestProgressive|TestBreak|TestBond|TestCompound|TestStraight|TestMarkup"

# Run basic tests only
test-basic: build
	go test -v ./tests/... -run "TestCompilationBasic|TestAdd|TestFactorial|TestGcd|TestPrime|TestFibonacci|TestCount|TestValidate|TestPassword|TestRoman|TestCredit|TestBusiness"

# Run nontrivial tests only
test-nontrivial: build
	go test -v ./tests/... -run "TestCompilationNontrivial|TestLevenshtein|TestExtended|TestBase64|TestRunLength|TestEaster|TestModular|TestLongest|TestCrC|TestAdler"

# Clean build artifacts
clean:
	rm -f tgpiler
	rm -rf examples/go

# Transpile all sample files to temp directory for inspection
transpile-all: build
	@mkdir -p /tmp/tgpiler-output
	@echo "Transpiling tsql_basic..."
	@for f in tsql_basic/*.sql; do \
		name=$$(basename "$$f" .sql); \
		./tgpiler "$$f" > "/tmp/tgpiler-output/basic_$$name.go"; \
	done
	@echo "Transpiling tsql_nontrivial..."
	@for f in tsql_nontrivial/*.sql; do \
		name=$$(basename "$$f" .sql); \
		./tgpiler "$$f" > "/tmp/tgpiler-output/nontrivial_$$name.go"; \
	done
	@echo "Transpiling tsql_financial..."
	@for f in tsql_financial/*.sql; do \
		name=$$(basename "$$f" .sql); \
		./tgpiler "$$f" > "/tmp/tgpiler-output/financial_$$name.go"; \
	done
	@echo "Output in /tmp/tgpiler-output/"

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Lint
lint:
	go vet ./...

# Show help
help:
	@echo "tgpiler - T-SQL to Go Transpiler"
	@echo ""
	@echo "Build & Install:"
	@echo "  make build            Build the transpiler binary"
	@echo "  make install          Install globally via go install"
	@echo "  make clean            Remove build artifacts"
	@echo ""
	@echo "Testing:"
	@echo "  make test             Run all tests (default)"
	@echo "  make test-quick       Quick smoke test (~5 tests)"
	@echo "  make test-compilation Verify SQL transpiles to valid Go syntax"
	@echo "  make test-e2e         Full E2E: transpile + compile + execute"
	@echo "  make test-e2e-compile E2E compile: verify go build on all files"
	@echo "  make test-e2e-execute E2E execute: run selected functions"
	@echo "  make test-basic       Run tsql_basic tests only"
	@echo "  make test-nontrivial  Run tsql_nontrivial tests only"
	@echo "  make test-financial   Run tsql_financial tests only"
	@echo ""
	@echo "Other:"
	@echo "  make transpile-all    Transpile all samples to /tmp"
	@echo "  make fmt              Format Go code"
	@echo "  make lint             Run go vet"
	@echo ""
	@echo "Scripts (in scripts/):"
	@echo "  ./scripts/test-all.sh        Run all tests with summary"
	@echo "  ./scripts/test-e2e.sh        Full E2E tests"
	@echo "  ./scripts/test-quick.sh      Quick smoke test"
	@echo "  ./scripts/transpile.sh FILE  Transpile and show output"
