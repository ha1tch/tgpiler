package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/tgpiler/protogen"
	"github.com/ha1tch/tgpiler/storage"
	"github.com/ha1tch/tgpiler/transpiler"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tgpiler", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		inputDir       = fs.String("d", "", "Read all .sql files from directory")
		inputDirL      = fs.String("dir", "", "Read all .sql files from directory")
		readStdin      = fs.Bool("s", false, "Read from stdin")
		readStdinL     = fs.Bool("stdin", false, "Read from stdin")
		output         = fs.String("o", "", "Write to single output file")
		outputL        = fs.String("output", "", "Write to single output file")
		outDir         = fs.String("O", "", "Write to output directory (creates if needed)")
		outDirL        = fs.String("outdir", "", "Write to output directory (creates if needed)")
		force          = fs.Bool("f", false, "Allow overwriting existing files")
		forceL         = fs.Bool("force", false, "Allow overwriting existing files")
		packageName    = fs.String("p", "main", "Package name for generated code")
		packageNameL   = fs.String("pkg", "main", "Package name for generated code")
		dmlMode        = fs.Bool("dml", false, "Enable DML mode (SELECT, INSERT, temp tables, etc.)")
		sqlDialect     = fs.String("dialect", "postgres", "SQL dialect (postgres, mysql, sqlite, sqlserver)")
		storeVar       = fs.String("store", "r.db", "Store variable name for DML operations")
		useSPLogger    = fs.Bool("splogger", false, "Use SPLogger for CATCH block error logging")
		spLoggerVar    = fs.String("logger", "spLogger", "SPLogger variable name")
		spLoggerType   = fs.String("logger-type", "slog", "SPLogger type: slog, db, file, multi, nop")
		spLoggerTable  = fs.String("logger-table", "Error.LogForStoreProcedure", "Table name for db logger")
		spLoggerFile   = fs.String("logger-file", "", "File path for file logger")
		spLoggerFormat = fs.String("logger-format", "json", "Format for file logger: json, text")
		genLoggerInit  = fs.Bool("logger-init", false, "Generate SPLogger initialization code")
		// Backend options
		backend       = fs.String("backend", "sql", "Backend type: sql, grpc, mock, inline")
		grpcClient    = fs.String("grpc-client", "client", "gRPC client variable name")
		grpcPackage   = fs.String("grpc-package", "", "Import path for generated gRPC package")
		mockStore     = fs.String("mock-store", "store", "Mock store variable name")
		// Proto/gRPC generation options
		protoFile     = fs.String("proto", "", "Proto file for gRPC operations")
		protoDir      = fs.String("proto-dir", "", "Directory of proto files")
		sqlDir        = fs.String("sql-dir", "", "Directory of SQL procedure files (for mapping)")
		serviceName   = fs.String("service", "", "Target service name (defaults to all)")
		genServer     = fs.Bool("gen-server", false, "Generate gRPC server stubs from proto")
		genImpl       = fs.Bool("gen-impl", false, "Generate repository implementations with procedure mappings")
		genMock       = fs.Bool("gen-mock", false, "Generate mock server code")
		showMappings  = fs.Bool("show-mappings", false, "Display procedure-to-method mappings")
		showHelp       = fs.Bool("h", false, "Show help")
		helpL          = fs.Bool("help", false, "Show help")
		showVer        = fs.Bool("v", false, "Show version")
		versionL       = fs.Bool("version", false, "Show version")
	)

	fs.Usage = func() {
		printUsage(stderr)
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Coalesce short and long flags
	if *inputDirL != "" {
		*inputDir = *inputDirL
	}
	if *readStdinL {
		*readStdin = true
	}
	if *outputL != "" {
		*output = *outputL
	}
	if *outDirL != "" {
		*outDir = *outDirL
	}
	if *forceL {
		*force = true
	}
	if *packageNameL != "main" {
		*packageName = *packageNameL
	}
	if *helpL {
		*showHelp = true
	}
	if *versionL {
		*showVer = true
	}

	if *showHelp {
		printUsage(stdout)
		return 0
	}

	if *showVer {
		fmt.Fprintf(stdout, "tgpiler version %s\n", version)
		return 0
	}

	// Determine input mode
	remainingArgs := fs.Args()
	inputFile := ""
	if len(remainingArgs) > 1 {
		fmt.Fprintln(stderr, "error: too many arguments")
		return 2
	}
	if len(remainingArgs) == 1 {
		inputFile = remainingArgs[0]
	}

	// Show help if no input specified (and not in proto generation mode)
	protoGenMode := *genServer || *genImpl || *genMock || *showMappings
	if inputFile == "" && *inputDir == "" && !*readStdin && !protoGenMode {
		printUsage(stdout)
		return 0
	}

	// Validate flag combinations
	if err := validateFlags(inputFile, *inputDir, *readStdin, *output, *outDir); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	// Execute based on mode
	cfg := &config{
		inputFile:      inputFile,
		inputDir:       *inputDir,
		readStdin:      *readStdin,
		output:         *output,
		outDir:         *outDir,
		force:          *force,
		packageName:    *packageName,
		dmlMode:        *dmlMode,
		sqlDialect:     *sqlDialect,
		storeVar:       *storeVar,
		useSPLogger:    *useSPLogger,
		spLoggerVar:    *spLoggerVar,
		spLoggerType:   *spLoggerType,
		spLoggerTable:  *spLoggerTable,
		spLoggerFile:   *spLoggerFile,
		spLoggerFormat: *spLoggerFormat,
		genLoggerInit:  *genLoggerInit,
		backend:        *backend,
		grpcClient:     *grpcClient,
		grpcPackage:    *grpcPackage,
		mockStore:      *mockStore,
		protoFile:      *protoFile,
		protoDir:       *protoDir,
		sqlDir:         *sqlDir,
		serviceName:    *serviceName,
		genServer:      *genServer,
		genImpl:        *genImpl,
		genMock:        *genMock,
		showMappings:   *showMappings,
		stdin:          stdin,
		stdout:         stdout,
		stderr:         stderr,
	}

	if err := execute(cfg); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

type config struct {
	inputFile      string
	inputDir       string
	readStdin      bool
	output         string
	outDir         string
	force          bool
	packageName    string
	dmlMode        bool
	sqlDialect     string
	storeVar       string
	useSPLogger    bool
	spLoggerVar    string
	spLoggerType   string
	spLoggerTable  string
	spLoggerFile   string
	spLoggerFormat string
	genLoggerInit  bool
	// Backend options
	backend     string
	grpcClient  string
	grpcPackage string
	mockStore   string
	// Proto/gRPC generation
	protoFile    string
	protoDir     string
	sqlDir       string
	serviceName  string
	genServer    bool
	genImpl      bool
	genMock      bool
	showMappings bool
	// IO
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func validateFlags(inputFile, inputDir string, readStdin bool, output, outDir string) error {
	// Check for conflicting input modes
	inputModes := 0
	if inputFile != "" {
		inputModes++
	}
	if inputDir != "" {
		inputModes++
	}
	if readStdin {
		inputModes++
	}
	if inputModes > 1 {
		return fmt.Errorf("cannot combine multiple input modes (file, --dir, --stdin)")
	}

	// outDir requires inputDir
	if outDir != "" && inputDir == "" {
		return fmt.Errorf("--outdir requires --dir (directory-to-directory mode)")
	}

	// Cannot combine output file and output directory
	if output != "" && outDir != "" {
		return fmt.Errorf("cannot specify both --output and --outdir")
	}

	return nil
}

func execute(cfg *config) error {
	// Proto generation modes (mutually exclusive with transpilation)
	if cfg.genServer || cfg.genImpl || cfg.genMock || cfg.showMappings {
		return executeProtoGen(cfg)
	}

	// Standard transpilation modes
	switch {
	case cfg.inputDir != "":
		return executeDirectory(cfg)
	case cfg.inputFile != "":
		return executeSingleFile(cfg)
	case cfg.readStdin:
		return executeStdin(cfg)
	default:
		return fmt.Errorf("no input specified")
	}
}

func executeStdin(cfg *config) error {
	source, err := io.ReadAll(cfg.stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	result, err := doTranspile(cfg, string(source))
	if err != nil {
		return err
	}

	return writeOutput(cfg, "", result)
}

func executeSingleFile(cfg *config) error {
	source, err := os.ReadFile(cfg.inputFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", cfg.inputFile, err)
	}

	result, err := doTranspile(cfg, string(source))
	if err != nil {
		return fmt.Errorf("%s: %w", cfg.inputFile, err)
	}

	return writeOutput(cfg, cfg.inputFile, result)
}

// doTranspile calls the appropriate transpiler based on config
func doTranspile(cfg *config, source string) (string, error) {
	if cfg.dmlMode {
		// Map backend string to BackendType
		var backendType transpiler.BackendType
		switch cfg.backend {
		case "sql":
			backendType = transpiler.BackendSQL
		case "grpc":
			backendType = transpiler.BackendGRPC
		case "mock":
			backendType = transpiler.BackendMock
		case "inline":
			backendType = transpiler.BackendInline
		default:
			return "", fmt.Errorf("unknown backend: %s (valid: sql, grpc, mock, inline)", cfg.backend)
		}

		dmlConfig := transpiler.DMLConfig{
			Backend:        backendType,
			SQLDialect:     cfg.sqlDialect,
			StoreVar:       cfg.storeVar,
			GRPCClientVar:  cfg.grpcClient,
			ProtoPackage:   cfg.grpcPackage,
			MockStoreVar:   cfg.mockStore,
			UseSPLogger:    cfg.useSPLogger,
			SPLoggerVar:    cfg.spLoggerVar,
			SPLoggerType:   cfg.spLoggerType,
			SPLoggerTable:  cfg.spLoggerTable,
			SPLoggerFile:   cfg.spLoggerFile,
			SPLoggerFormat: cfg.spLoggerFormat,
			GenLoggerInit:  cfg.genLoggerInit,
		}
		return transpiler.TranspileWithDML(source, cfg.packageName, dmlConfig)
	}
	return transpiler.Transpile(source, cfg.packageName)
}

func executeDirectory(cfg *config) error {
	entries, err := os.ReadDir(cfg.inputDir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", cfg.inputDir, err)
	}

	// Create output directory if needed
	if cfg.outDir != "" {
		if err := os.MkdirAll(cfg.outDir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			continue
		}

		inputPath := filepath.Join(cfg.inputDir, entry.Name())
		source, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", inputPath, err)
		}

		result, err := doTranspile(cfg, string(source))
		if err != nil {
			return fmt.Errorf("%s: %w", inputPath, err)
		}

		if cfg.outDir != "" {
			outName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) + ".go"
			outPath := filepath.Join(cfg.outDir, outName)

			if !cfg.force {
				if _, err := os.Stat(outPath); err == nil {
					return fmt.Errorf("output file %s already exists (use --force to overwrite)", outPath)
				}
			}

			if err := os.WriteFile(outPath, []byte(result), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}
			fmt.Fprintf(cfg.stderr, "%s -> %s\n", inputPath, outPath)
		} else {
			fmt.Fprintln(cfg.stdout, result)
		}
	}

	return nil
}

func writeOutput(cfg *config, inputPath, content string) error {
	if cfg.output != "" {
		if !cfg.force {
			if _, err := os.Stat(cfg.output); err == nil {
				return fmt.Errorf("output file %s already exists (use --force to overwrite)", cfg.output)
			}
		}
		if err := os.WriteFile(cfg.output, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", cfg.output, err)
		}
		return nil
	}

	fmt.Fprint(cfg.stdout, content)
	return nil
}

// executeProtoGen handles proto-based code generation modes
func executeProtoGen(cfg *config) error {
	// Parse proto files
	proto, err := parseProtoFiles(cfg)
	if err != nil {
		return err
	}

	// Parse SQL procedures if needed for mapping
	var procedures []*storage.Procedure
	if cfg.genImpl || cfg.showMappings {
		procedures, err = parseSQLProcedures(cfg)
		if err != nil {
			return err
		}
	}

	// Execute requested generation
	if cfg.showMappings {
		return showMappings(cfg, proto, procedures)
	}
	if cfg.genServer {
		return generateServer(cfg, proto)
	}
	if cfg.genImpl {
		return generateImpl(cfg, proto, procedures)
	}
	if cfg.genMock {
		return generateMock(cfg, proto)
	}

	return nil
}

// parseProtoFiles parses .proto files from file or directory
func parseProtoFiles(cfg *config) (*storage.ProtoParseResult, error) {
	parser := protogen.NewParser()

	if cfg.protoDir != "" {
		return parser.ParseDir(cfg.protoDir)
	}
	if cfg.protoFile != "" {
		return parser.ParseFiles(cfg.protoFile)
	}
	return nil, fmt.Errorf("no proto file specified (use --proto or --proto-dir)")
}

// parseSQLProcedures parses SQL files and extracts procedure info
func parseSQLProcedures(cfg *config) ([]*storage.Procedure, error) {
	sqlDir := cfg.sqlDir
	if sqlDir == "" {
		sqlDir = cfg.inputDir
	}
	if sqlDir == "" && cfg.inputFile != "" {
		// Single file mode
		return parseSQLFile(cfg.inputFile)
	}
	if sqlDir == "" {
		return nil, fmt.Errorf("no SQL directory specified (use --sql-dir or --dir)")
	}

	var allProcs []*storage.Procedure
	entries, err := os.ReadDir(sqlDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", sqlDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			continue
		}
		procs, err := parseSQLFile(filepath.Join(sqlDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		allProcs = append(allProcs, procs...)
	}

	return allProcs, nil
}

// parseSQLFile parses a single SQL file and extracts procedures
func parseSQLFile(path string) ([]*storage.Procedure, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	extractor := storage.NewProcedureExtractor()
	procs, err := extractor.ExtractAll(string(source))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return procs, nil
}

// showMappings displays procedure-to-method mappings
func showMappings(cfg *config, proto *storage.ProtoParseResult, procedures []*storage.Procedure) error {
	mapper := storage.NewEnsembleMapper(proto, procedures)
	mappings := mapper.MapAll()
	stats := mapper.GetStats()

	fmt.Fprintf(cfg.stdout, "Procedure-to-Method Mappings\n")
	fmt.Fprintf(cfg.stdout, "============================\n\n")

	// Group by service
	serviceMethodMappings := make(map[string][]string)
	for key, mapping := range mappings {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 {
			continue
		}
		svcName := parts[0]
		methodName := parts[1]

		line := fmt.Sprintf("  %s -> %s (%.0f%% confidence, %s)",
			methodName,
			mapping.Procedure.Name,
			mapping.Confidence*100,
			mapping.MatchReason)
		serviceMethodMappings[svcName] = append(serviceMethodMappings[svcName], line)
	}

	for svcName, methods := range serviceMethodMappings {
		fmt.Fprintf(cfg.stdout, "Service: %s\n", svcName)
		for _, line := range methods {
			fmt.Fprintln(cfg.stdout, line)
		}
		fmt.Fprintln(cfg.stdout)
	}

	fmt.Fprintf(cfg.stdout, "Statistics:\n")
	fmt.Fprintf(cfg.stdout, "  Total methods: %d\n", stats.TotalMethods)
	fmt.Fprintf(cfg.stdout, "  Mapped: %d\n", stats.MappedMethods)
	fmt.Fprintf(cfg.stdout, "  Unmapped: %d\n", stats.UnmappedMethods)
	fmt.Fprintf(cfg.stdout, "  High confidence (>80%%): %d\n", stats.HighConfidence)
	fmt.Fprintf(cfg.stdout, "  Medium confidence (50-80%%): %d\n", stats.MediumConfidence)
	fmt.Fprintf(cfg.stdout, "  Low confidence (<50%%): %d\n", stats.LowConfidence)

	return nil
}

// generateServer generates gRPC server stubs
func generateServer(cfg *config, proto *storage.ProtoParseResult) error {
	opts := protogen.DefaultServerGenOptions()
	opts.PackageName = cfg.packageName

	gen := protogen.NewServerGenerator(proto, opts)

	var buf bytes.Buffer
	if cfg.serviceName != "" {
		if err := gen.GenerateService(cfg.serviceName, &buf); err != nil {
			return err
		}
	} else {
		if err := gen.GenerateAll(&buf); err != nil {
			return err
		}
	}

	return writeOutput(cfg, "", buf.String())
}

// generateImpl generates repository implementations with procedure mappings
func generateImpl(cfg *config, proto *storage.ProtoParseResult, procedures []*storage.Procedure) error {
	gen := protogen.NewImplementationGenerator(proto, procedures)

	opts := protogen.DefaultServerGenOptions()
	opts.PackageName = cfg.packageName

	var buf bytes.Buffer
	if cfg.serviceName != "" {
		if err := gen.GenerateServiceImpl(cfg.serviceName, opts, &buf); err != nil {
			return err
		}
	} else {
		// Generate for all services
		for svcName := range proto.AllServices {
			if err := gen.GenerateServiceImpl(svcName, opts, &buf); err != nil {
				return err
			}
			buf.WriteString("\n")
		}
	}

	// Show mapping stats
	stats := gen.GetStats()
	fmt.Fprintf(cfg.stderr, "Generated implementations: %d methods mapped, %d unmapped\n",
		stats.MappedMethods, stats.UnmappedMethods)

	return writeOutput(cfg, "", buf.String())
}

// generateMock generates mock server code
func generateMock(cfg *config, proto *storage.ProtoParseResult) error {
	var buf bytes.Buffer

	// Generate mock server code
	buf.WriteString(fmt.Sprintf("// Code generated by tgpiler. DO NOT EDIT.\n\n"))
	buf.WriteString(fmt.Sprintf("package %s\n\n", cfg.packageName))
	buf.WriteString("import (\n")
	buf.WriteString("\t\"github.com/ha1tch/tgpiler/protogen\"\n")
	buf.WriteString("\t\"github.com/ha1tch/tgpiler/storage\"\n")
	buf.WriteString(")\n\n")

	// Generate helper to create mock server
	buf.WriteString("// NewMockServer creates a mock server with default handlers.\n")
	buf.WriteString("func NewMockServer(proto *storage.ProtoParseResult) *protogen.MockServer {\n")
	buf.WriteString("\treturn protogen.NewMockServer(proto)\n")
	buf.WriteString("}\n\n")

	// List services and methods
	buf.WriteString("/*\nAvailable services and methods:\n\n")
	for svcName, svc := range proto.AllServices {
		buf.WriteString(fmt.Sprintf("Service: %s\n", svcName))
		for _, method := range svc.Methods {
			buf.WriteString(fmt.Sprintf("  - %s(%s) -> %s\n",
				method.Name, method.RequestType, method.ResponseType))
		}
		buf.WriteString("\n")
	}
	buf.WriteString("*/\n")

	return writeOutput(cfg, "", buf.String())
}



func printUsage(w io.Writer) {
	fmt.Fprint(w, `tgpiler - T-SQL to Go transpiler

Usage:
  tgpiler [options] <input.sql>
  tgpiler [options] -s < input.sql
  tgpiler [options] -d <path>
  tgpiler --gen-server --proto <file> [options]
  tgpiler --gen-server --proto-dir <path> [options]
  tgpiler --gen-impl --proto-dir <path> --sql-dir <path> [options]

Input (mutually exclusive):
  <file.sql>            Read single file
  -s, --stdin           Read from stdin
  -d, --dir <path>      Read all .sql files from directory

Output (mutually exclusive):
  (no flag)             Write to stdout
  -o, --output <file>   Write to single file
  -O, --outdir <path>   Write to directory (creates if needed)

General Options:
  -p, --pkg <n>         Package name for generated code (default: main)
  --dml                 Enable DML mode (SELECT, INSERT, temp tables, JSON/XML)
  --dialect <n>         SQL dialect: postgres, mysql, sqlite, sqlserver (default: postgres)
  --store <var>         Store variable name (default: r.db)
  -f, --force           Allow overwriting existing files
  -h, --help            Show help
  -v, --version         Show version

Backend Options (requires --dml):
  --backend <type>      Backend: sql, grpc, mock, inline (default: sql)
  --grpc-client <var>   gRPC client variable name (default: client)
  --grpc-package <path> Import path for generated gRPC package
  --mock-store <var>    Mock store variable name (default: store)

Proto/gRPC Generation (mutually exclusive with transpilation):
  --proto <file>        Proto file for gRPC operations
  --proto-dir <path>    Directory of proto files
  --sql-dir <path>      Directory of SQL procedure files (for mapping)
  --service <name>      Target service name (defaults to all)
  --gen-server          Generate gRPC server stubs from proto
  --gen-impl            Generate repository implementations with procedure mappings
  --gen-mock            Generate mock server code
  --show-mappings       Display procedure-to-method mappings

SPLogger Options (requires --dml):
  --splogger            Enable SPLogger for CATCH block error logging
  --logger <var>        SPLogger variable name (default: spLogger)
  --logger-type <type>  Logger type: slog, db, file, multi, nop (default: slog)
  --logger-table <n>    Table name for db logger (default: Error.LogForStoreProcedure)
  --logger-file <path>  File path for file logger
  --logger-format <f>   Format for file logger: json, text (default: json)
  --logger-init         Generate SPLogger initialization code

Examples:
  # Basic transpilation
  tgpiler input.sql                       # file to stdout
  tgpiler --dml input.sql                 # with DML support

  # Using gRPC backend
  tgpiler --dml --backend=grpc --grpc-client=svc input.sql

  # Using mock backend
  tgpiler --dml --backend=mock --mock-store=mockDB input.sql

  # Generate server stubs from proto (single file or directory)
  tgpiler --gen-server --proto api.proto -o server.go
  tgpiler --gen-server --proto-dir ./protos -o all_servers.go

  # Generate repository implementations with procedure mappings
  tgpiler --gen-impl --proto-dir ./protos --sql-dir ./procedures -o repo.go

  # Show procedure-to-method mappings
  tgpiler --show-mappings --proto-dir ./protos --sql-dir ./procedures

  # SPLogger with slog (default)
  tgpiler --dml --splogger input.sql

  # SPLogger with database logging
  tgpiler --dml --splogger --logger-type=db --logger-table=ErrorLog input.sql

  # Directory processing
  tgpiler -d ./sql -O ./go                # directory to directory
  tgpiler -d ./sql -O ./go -f             # with overwrite

Exit codes:
  0  Success
  1  Parse/transpile error
  2  CLI usage error
`)
}