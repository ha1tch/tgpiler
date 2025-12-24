package main

import (
	"bytes"
	"encoding/json"
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

// annotateFlag is a custom flag that supports both --annotate and --annotate=level
// Levels: none, minimal, standard, verbose
type annotateFlag struct {
	level string
}

func (f *annotateFlag) String() string {
	if f.level == "" {
		return "none"
	}
	return f.level
}

func (f *annotateFlag) Set(s string) error {
	// --annotate without value, or --annotate=true
	if s == "" || s == "true" {
		f.level = "standard"
		return nil
	}
	// Validate level
	switch s {
	case "none", "minimal", "standard", "verbose":
		f.level = s
		return nil
	default:
		return fmt.Errorf("invalid annotate level %q: must be none, minimal, standard, or verbose", s)
	}
}

func (f *annotateFlag) IsBoolFlag() bool {
	// This allows --annotate without =value
	return true
}

func (f *annotateFlag) Level() string {
	if f.level == "" {
		return "none"
	}
	return f.level
}

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
		receiver       = fs.String("receiver", "r", "Receiver variable name for generated methods (empty for standalone functions)")
		receiverType   = fs.String("receiver-type", "*Repository", "Receiver type for generated methods")
		preserveGo     = fs.Bool("preserve-go", false, "Don't strip GO batch separators (default: strip them)")
		sequenceMode   = fs.String("sequence-mode", "db", "Sequence handling: db, uuid, stub (default: db)")
		newidMode      = fs.String("newid", "app", "NEWID() handling: app, db, grpc, mock, stub (default: app)")
		idServiceVar   = fs.String("id-service", "", "gRPC client variable for --newid=grpc")
		skipDDL        = fs.Bool("skip-ddl", true, "Skip DDL statements with warning (default: true)")
		strictDDL      = fs.Bool("strict-ddl", false, "Fail on any DDL statement")
		extractDDL     = fs.String("extract-ddl", "", "Extract skipped DDL to separate file")
		useSPLogger    = fs.Bool("splogger", false, "Use SPLogger for CATCH block error logging")
		spLoggerVar    = fs.String("logger", "spLogger", "SPLogger variable name")
		spLoggerType   = fs.String("logger-type", "slog", "SPLogger type: slog, db, file, multi, nop")
		spLoggerTable  = fs.String("logger-table", "Error.LogForStoreProcedure", "Table name for db logger")
		spLoggerFile   = fs.String("logger-file", "", "File path for file logger")
		spLoggerFormat = fs.String("logger-format", "json", "Format for file logger: json, text")
		genLoggerInit  = fs.Bool("logger-init", false, "Generate SPLogger initialization code")
		// Backend options
		backend         = fs.String("backend", "sql", "Backend type: sql, grpc, mock, inline")
		fallbackBackend = fs.String("fallback-backend", "", "Fallback backend for temp tables: sql, mock (default: sql)")
		grpcClient      = fs.String("grpc-client", "client", "gRPC client variable name")
		grpcPackage   = fs.String("grpc-package", "", "Import path for generated gRPC package")
		mockStore     = fs.String("mock-store", "store", "Mock store variable name")
		// gRPC mapping options
		tableService  = fs.String("table-service", "", "Table-to-service mappings (format: Table:Service,Table:Service)")
		tableClient   = fs.String("table-client", "", "Table-to-client mappings (format: Table:client,Table:client)")
		grpcMappings  = fs.String("grpc-mappings", "", "Procedure-to-method mappings (format: proc:Service.Method,proc:Service.Method)")
		// Proto/gRPC generation options
		protoFile     = fs.String("proto", "", "Proto file for gRPC operations")
		protoDir      = fs.String("proto-dir", "", "Directory of proto files")
		sqlDir        = fs.String("sql-dir", "", "Directory of SQL procedure files (for mapping)")
		serviceName   = fs.String("service", "", "Target service name (defaults to all)")
		genServer     = fs.Bool("gen-server", false, "Generate gRPC server stubs from proto")
		genImpl       = fs.Bool("gen-impl", false, "Generate repository implementations with procedure mappings")
		genMock       = fs.Bool("gen-mock", false, "Generate mock server code")
		showMappings  = fs.Bool("show-mappings", false, "Display procedure-to-method mappings")
		outputFormat  = fs.String("output-format", "text", "Output format for --show-mappings (text, json, markdown, html)")
		warnThreshold = fs.Int("warn-threshold", 50, "Confidence threshold (%) for low-confidence warnings (0-100)")
		showHelp       = fs.Bool("h", false, "Show help")
		helpL          = fs.Bool("help", false, "Show help")
		showVer        = fs.Bool("v", false, "Show version")
		versionL       = fs.Bool("version", false, "Show version")
	)
	
	// Custom flag for --annotate / --annotate=level
	var annotate annotateFlag
	fs.Var(&annotate, "annotate", "Add code annotations (levels: none, minimal, standard, verbose; default if flag present: standard)")

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
		receiver:       *receiver,
		receiverType:   *receiverType,
		preserveGo:     *preserveGo,
		sequenceMode:   *sequenceMode,
		newidMode:      *newidMode,
		idServiceVar:   *idServiceVar,
		skipDDL:        *skipDDL,
		strictDDL:      *strictDDL,
		extractDDL:      *extractDDL,
		useSPLogger:     *useSPLogger,
		spLoggerVar:     *spLoggerVar,
		spLoggerType:    *spLoggerType,
		spLoggerTable:   *spLoggerTable,
		spLoggerFile:    *spLoggerFile,
		spLoggerFormat:  *spLoggerFormat,
		genLoggerInit:   *genLoggerInit,
		backend:         *backend,
		fallbackBackend: *fallbackBackend,
		grpcClient:      *grpcClient,
		grpcPackage:    *grpcPackage,
		mockStore:      *mockStore,
		tableService:   *tableService,
		tableClient:    *tableClient,
		grpcMappings:   *grpcMappings,
		protoFile:      *protoFile,
		protoDir:       *protoDir,
		sqlDir:         *sqlDir,
		serviceName:    *serviceName,
		genServer:      *genServer,
		genImpl:        *genImpl,
		genMock:        *genMock,
		showMappings:   *showMappings,
		outputFormat:   *outputFormat,
		warnThreshold:  *warnThreshold,
		annotateLevel:  annotate.Level(),
		stdin:          stdin,
		stdout:         stdout,
		stderr:         stderr,
	}

	if err := execute(cfg); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// Write extracted DDL to file if configured
	if cfg.extractDDL != "" && len(cfg.collectedDDL) > 0 {
		ddlContent := "-- DDL statements extracted by tgpiler\n"
		ddlContent += "-- These should be kept in your database schema/migrations\n\n"
		for _, ddl := range cfg.collectedDDL {
			ddlContent += ddl + ";\nGO\n\n"
		}
		if err := os.WriteFile(cfg.extractDDL, []byte(ddlContent), 0644); err != nil {
			fmt.Fprintf(stderr, "error writing DDL file: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "Extracted %d DDL statements to %s\n", len(cfg.collectedDDL), cfg.extractDDL)
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
	receiver       string
	receiverType   string
	preserveGo     bool
	sequenceMode   string
	newidMode      string
	idServiceVar   string
	skipDDL        bool
	strictDDL      bool
	extractDDL     string
	collectedDDL   []string // Accumulated DDL statements for extraction
	useSPLogger    bool
	spLoggerVar    string
	spLoggerType   string
	spLoggerTable  string
	spLoggerFile   string
	spLoggerFormat string
	genLoggerInit  bool
	// Backend options
	backend         string
	fallbackBackend string
	grpcClient      string
	grpcPackage  string
	mockStore    string
	tableService string
	tableClient  string
	grpcMappings string
	// Proto/gRPC generation
	protoFile    string
	protoDir     string
	sqlDir       string
	serviceName   string
	genServer     bool
	genImpl       bool
	genMock       bool
	showMappings  bool
	outputFormat  string
	warnThreshold int
	annotateLevel string
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

// parseMapping parses a comma-separated mapping string into a map.
// Format: "key:value,key:value" or "key=value,key=value"
// Returns nil if input is empty.
func parseMapping(s string) map[string]string {
	if s == "" {
		return nil
	}
	result := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		// Support both : and = as separators
		var key, value string
		if idx := strings.Index(pair, ":"); idx > 0 {
			key = strings.TrimSpace(pair[:idx])
			value = strings.TrimSpace(pair[idx+1:])
		} else if idx := strings.Index(pair, "="); idx > 0 {
			key = strings.TrimSpace(pair[:idx])
			value = strings.TrimSpace(pair[idx+1:])
		} else {
			continue // Invalid format, skip
		}
		if key != "" && value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
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

		// Map fallback backend string to BackendType
		var fallbackBackendType transpiler.BackendType
		fallbackExplicit := cfg.fallbackBackend != ""
		switch cfg.fallbackBackend {
		case "sql", "":
			fallbackBackendType = transpiler.BackendSQL
		case "mock":
			fallbackBackendType = transpiler.BackendMock
		default:
			return "", fmt.Errorf("unknown fallback-backend: %s (valid: sql, mock)", cfg.fallbackBackend)
		}

		dmlConfig := transpiler.DMLConfig{
			Backend:          backendType,
			FallbackBackend:  fallbackBackendType,
			FallbackExplicit: fallbackExplicit,
			SQLDialect:       cfg.sqlDialect,
			StoreVar:         cfg.storeVar,
			Receiver:         cfg.receiver,
			ReceiverType:     cfg.receiverType,
			PreserveGo:       cfg.preserveGo,
			SequenceMode:     cfg.sequenceMode,
			NewidMode:        cfg.newidMode,
			IDServiceVar:     cfg.idServiceVar,
			SkipDDL:          cfg.skipDDL,
			StrictDDL:        cfg.strictDDL,
			ExtractDDL:       cfg.extractDDL,
			GRPCClientVar:    cfg.grpcClient,
			ProtoPackage:     cfg.grpcPackage,
			MockStoreVar:     cfg.mockStore,
			TableToService:   parseMapping(cfg.tableService),
			TableToClient:    parseMapping(cfg.tableClient),
			GRPCMappings:     parseMapping(cfg.grpcMappings),
			ServiceToPackage: make(map[string]string),
			UseSPLogger:      cfg.useSPLogger,
			SPLoggerVar:      cfg.spLoggerVar,
			SPLoggerType:     cfg.spLoggerType,
			SPLoggerTable:    cfg.spLoggerTable,
			SPLoggerFile:     cfg.spLoggerFile,
			SPLoggerFormat:   cfg.spLoggerFormat,
			GenLoggerInit:    cfg.genLoggerInit,
			AnnotateLevel:    cfg.annotateLevel,
		}
		
		// Use extended result to capture DDL for extraction
		result, err := transpiler.TranspileWithDMLEx(source, cfg.packageName, dmlConfig)
		if err != nil {
			return "", err
		}
		
		// Accumulate extracted DDL for later file writing
		if cfg.extractDDL != "" && len(result.ExtractedDDL) > 0 {
			cfg.collectedDDL = append(cfg.collectedDDL, result.ExtractedDDL...)
		}
		
		// Print DDL warnings to stderr
		for _, warning := range result.DDLWarnings {
			fmt.Fprintf(cfg.stderr, "warning: %s\n", warning)
		}
		
		// Print temp table warnings to stderr
		for _, warning := range result.TempTableWarnings {
			fmt.Fprintf(cfg.stderr, "info: %s\n", warning)
		}
		
		return result.Code, nil
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

	switch cfg.outputFormat {
	case "json":
		return showMappingsJSON(cfg, mappings, stats, procedures)
	case "markdown", "md":
		return showMappingsMarkdown(cfg, mappings, stats, procedures)
	case "html":
		return showMappingsHTML(cfg, mappings, stats, procedures)
	default:
		return showMappingsText(cfg, mappings, stats, procedures)
	}
}

// MappingData represents the JSON output structure
type MappingData struct {
	Services   []ServiceMappingData `json:"services"`
	Statistics MappingStats         `json:"statistics"`
	Unmapped   []string             `json:"unmapped_procedures"`
}

type ServiceMappingData struct {
	Name     string          `json:"name"`
	Mappings []MethodMapping `json:"mappings"`
}

type MethodMapping struct {
	RPC        string   `json:"rpc"`
	Procedure  string   `json:"procedure"`
	Confidence float64  `json:"confidence"`
	Signals    []string `json:"signals"`
	Warnings   []string `json:"warnings,omitempty"`
}

type MappingStats struct {
	Total            int `json:"total"`
	Mapped           int `json:"mapped"`
	Unmapped         int `json:"unmapped"`
	HighConfidence   int `json:"high_confidence"`
	MediumConfidence int `json:"medium_confidence"`
	LowConfidence    int `json:"low_confidence"`
}

func showMappingsText(cfg *config, mappings map[string]*storage.MethodMapping, stats storage.MappingStats, procedures []*storage.Procedure) error {
	fmt.Fprintf(cfg.stdout, "Procedure-to-Method Mappings\n")
	fmt.Fprintf(cfg.stdout, "============================\n\n")

	// Collect low-confidence mappings for warnings
	type lowConfMapping struct {
		key        string
		methodName string
		mapping    *storage.MethodMapping
	}
	var lowConfMappings []lowConfMapping

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

		// Track low-confidence mappings
		threshold := float64(cfg.warnThreshold) / 100.0
		if mapping.Confidence < threshold {
			lowConfMappings = append(lowConfMappings, lowConfMapping{
				key:        key,
				methodName: methodName,
				mapping:    mapping,
			})
		}
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

	// Find unmapped procedures
	mappedProcs := make(map[string]bool)
	for _, m := range mappings {
		mappedProcs[m.Procedure.Name] = true
	}
	
	var unmapped []string
	for _, p := range procedures {
		if !mappedProcs[p.Name] {
			unmapped = append(unmapped, p.Name)
		}
	}

	// Show actionable warnings for low-confidence mappings
	if len(lowConfMappings) > 0 {
		fmt.Fprintf(cfg.stdout, "\nLow-Confidence Warnings (%d):\n", len(lowConfMappings))
		fmt.Fprintf(cfg.stdout, "  These mappings may be incorrect and should be reviewed:\n\n")
		
		for _, lc := range lowConfMappings {
			fmt.Fprintf(cfg.stdout, "  WARNING: %s -> %s (%.0f%% confidence)\n",
				lc.methodName, lc.mapping.Procedure.Name, lc.mapping.Confidence*100)
			
			// Find potential alternatives from unmapped procedures
			alternatives := findAlternatives(lc.methodName, unmapped, 3)
			if len(alternatives) > 0 {
				fmt.Fprintf(cfg.stdout, "    Possible alternatives from unmapped procedures:\n")
				for _, alt := range alternatives {
					fmt.Fprintf(cfg.stdout, "      - %s\n", alt)
				}
			}
			
			// Show override syntax
			fmt.Fprintf(cfg.stdout, "    To override: --grpc-mappings=\"%s:%s\"\n\n",
				lc.mapping.Procedure.Name, lc.key)
		}
	}

	if len(unmapped) > 0 {
		fmt.Fprintf(cfg.stdout, "\nUnmapped Procedures (%d):\n", len(unmapped))
		fmt.Fprintf(cfg.stdout, "  These stored procedures have no matching RPC method:\n")
		for _, name := range unmapped {
			fmt.Fprintf(cfg.stdout, "  - %s\n", name)
		}
	}

	return nil
}

// findAlternatives finds procedures that might be alternatives based on name similarity
func findAlternatives(methodName string, procedures []string, maxResults int) []string {
	// Split before lowercasing to preserve camelCase boundaries
	methodWords := splitWords(methodName)
	for i := range methodWords {
		methodWords[i] = strings.ToLower(methodWords[i])
	}
	
	type scored struct {
		name  string
		score int
	}
	var candidates []scored
	
	for _, proc := range procedures {
		// Remove usp_ prefix for comparison
		procClean := strings.TrimPrefix(proc, "usp_")
		procWords := splitWords(procClean)
		for i := range procWords {
			procWords[i] = strings.ToLower(procWords[i])
		}
		
		score := 0
		
		// Check for substring match (full name)
		methodLower := strings.ToLower(methodName)
		procLower := strings.ToLower(procClean)
		if strings.Contains(procLower, methodLower) || strings.Contains(methodLower, procLower) {
			score += 3
		}
		
		// Check for word overlap
		for _, mw := range methodWords {
			for _, pw := range procWords {
				if mw == pw {
					score += 2
				} else if len(mw) > 2 && len(pw) > 2 && (strings.HasPrefix(pw, mw) || strings.HasPrefix(mw, pw)) {
					score += 1
				}
			}
		}
		
		if score > 0 {
			candidates = append(candidates, scored{proc, score})
		}
	}
	
	// Sort by score descending
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	
	// Return top N
	var results []string
	for i := 0; i < len(candidates) && i < maxResults; i++ {
		results = append(results, candidates[i].name)
	}
	return results
}

// splitWords splits a camelCase or snake_case string into words
func splitWords(s string) []string {
	// Handle snake_case
	s = strings.ReplaceAll(s, "_", " ")
	
	// Handle camelCase
	var words []string
	var current strings.Builder
	for i, r := range s {
		if r == ' ' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		} else if i > 0 && r >= 'A' && r <= 'Z' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			current.WriteRune(r + 32) // lowercase
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

func showMappingsJSON(cfg *config, mappings map[string]*storage.MethodMapping, stats storage.MappingStats, procedures []*storage.Procedure) error {
	data := MappingData{
		Statistics: MappingStats{
			Total:            stats.TotalMethods,
			Mapped:           stats.MappedMethods,
			Unmapped:         stats.UnmappedMethods,
			HighConfidence:   stats.HighConfidence,
			MediumConfidence: stats.MediumConfidence,
			LowConfidence:    stats.LowConfidence,
		},
	}

	// Group by service
	serviceMap := make(map[string]*ServiceMappingData)
	for key, mapping := range mappings {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 {
			continue
		}
		svcName := parts[0]
		methodName := parts[1]

		if _, ok := serviceMap[svcName]; !ok {
			serviceMap[svcName] = &ServiceMappingData{Name: svcName}
		}

		mm := MethodMapping{
			RPC:        methodName,
			Procedure:  mapping.Procedure.Name,
			Confidence: mapping.Confidence,
			Signals:    strings.Split(mapping.MatchReason, "; "),
		}
		if mapping.Confidence < 0.5 {
			mm.Warnings = append(mm.Warnings, "Low confidence - manual review recommended")
		}
		serviceMap[svcName].Mappings = append(serviceMap[svcName].Mappings, mm)
	}

	for _, svc := range serviceMap {
		data.Services = append(data.Services, *svc)
	}

	// Find unmapped procedures
	mappedProcs := make(map[string]bool)
	for _, m := range mappings {
		mappedProcs[m.Procedure.Name] = true
	}
	for _, p := range procedures {
		if !mappedProcs[p.Name] {
			data.Unmapped = append(data.Unmapped, p.Name)
		}
	}

	enc := json.NewEncoder(cfg.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func showMappingsMarkdown(cfg *config, mappings map[string]*storage.MethodMapping, stats storage.MappingStats, procedures []*storage.Procedure) error {
	fmt.Fprintf(cfg.stdout, "# Procedure-to-Method Mappings\n\n")

	// Statistics summary
	fmt.Fprintf(cfg.stdout, "## Summary\n\n")
	fmt.Fprintf(cfg.stdout, "| Metric | Count |\n")
	fmt.Fprintf(cfg.stdout, "|--------|-------|\n")
	fmt.Fprintf(cfg.stdout, "| Total Methods | %d |\n", stats.TotalMethods)
	fmt.Fprintf(cfg.stdout, "| Mapped | %d |\n", stats.MappedMethods)
	fmt.Fprintf(cfg.stdout, "| Unmapped | %d |\n", stats.UnmappedMethods)
	fmt.Fprintf(cfg.stdout, "| High Confidence (>80%%) | %d |\n", stats.HighConfidence)
	fmt.Fprintf(cfg.stdout, "| Medium Confidence (50-80%%) | %d |\n", stats.MediumConfidence)
	fmt.Fprintf(cfg.stdout, "| Low Confidence (<50%%) | %d |\n\n", stats.LowConfidence)

	// Group by service
	serviceMethodMappings := make(map[string][]*storage.MethodMapping)
	serviceMethodNames := make(map[string][]string)
	for key, mapping := range mappings {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 {
			continue
		}
		svcName := parts[0]
		methodName := parts[1]
		serviceMethodMappings[svcName] = append(serviceMethodMappings[svcName], mapping)
		serviceMethodNames[svcName] = append(serviceMethodNames[svcName], methodName)
	}

	fmt.Fprintf(cfg.stdout, "## Mappings by Service\n\n")
	for svcName, mappingList := range serviceMethodMappings {
		fmt.Fprintf(cfg.stdout, "### %s\n\n", svcName)
		fmt.Fprintf(cfg.stdout, "| RPC Method | Stored Procedure | Confidence | Match Reason |\n")
		fmt.Fprintf(cfg.stdout, "|------------|------------------|------------|-------------|\n")
		names := serviceMethodNames[svcName]
		for i, mapping := range mappingList {
			confStr := fmt.Sprintf("%.0f%%", mapping.Confidence*100)
			confIcon := "🟢"
			if mapping.Confidence < 0.5 {
				confIcon = "🔴"
			} else if mapping.Confidence < 0.8 {
				confIcon = "🟡"
			}
			fmt.Fprintf(cfg.stdout, "| %s | %s | %s %s | %s |\n",
				names[i], mapping.Procedure.Name, confIcon, confStr, mapping.MatchReason)
		}
		fmt.Fprintln(cfg.stdout)
	}

	// Unmapped procedures
	mappedProcs := make(map[string]bool)
	for _, m := range mappings {
		mappedProcs[m.Procedure.Name] = true
	}
	var unmapped []string
	for _, p := range procedures {
		if !mappedProcs[p.Name] {
			unmapped = append(unmapped, p.Name)
		}
	}
	if len(unmapped) > 0 {
		fmt.Fprintf(cfg.stdout, "## Unmapped Procedures\n\n")
		fmt.Fprintf(cfg.stdout, "The following %d procedures have no matching RPC method:\n\n", len(unmapped))
		for _, name := range unmapped {
			fmt.Fprintf(cfg.stdout, "- `%s`\n", name)
		}
	}

	return nil
}

func showMappingsHTML(cfg *config, mappings map[string]*storage.MethodMapping, stats storage.MappingStats, procedures []*storage.Procedure) error {
	// Group by service first
	serviceMethodMappings := make(map[string][]*storage.MethodMapping)
	serviceMethodNames := make(map[string][]string)
	for key, mapping := range mappings {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 {
			continue
		}
		svcName := parts[0]
		methodName := parts[1]
		serviceMethodMappings[svcName] = append(serviceMethodMappings[svcName], mapping)
		serviceMethodNames[svcName] = append(serviceMethodNames[svcName], methodName)
	}

	// Find unmapped procedures
	mappedProcs := make(map[string]bool)
	for _, m := range mappings {
		mappedProcs[m.Procedure.Name] = true
	}
	var unmapped []string
	for _, p := range procedures {
		if !mappedProcs[p.Name] {
			unmapped = append(unmapped, p.Name)
		}
	}

	fmt.Fprintf(cfg.stdout, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>tgpiler Mapping Report</title>
<style>
:root { --bg: #f8f9fa; --card: #fff; --text: #1a1a2e; --border: rgba(0,0,0,0.1); --hover: rgba(0,0,0,0.05); --green: #16a34a; --yellow: #ca8a04; --red: #dc2626; --blue: #2563eb; }
@media (prefers-color-scheme: dark) {
  :root { --bg: #1a1a2e; --card: #16213e; --text: #eee; --border: rgba(255,255,255,0.1); --hover: rgba(255,255,255,0.1); --green: #4ade80; --yellow: #fbbf24; --red: #f87171; --blue: #60a5fa; }
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: system-ui, -apple-system, sans-serif; background: var(--bg); color: var(--text); padding: 2rem; }
h1 { margin-bottom: 1.5rem; }
.stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
.stat-card { background: var(--card); padding: 1rem; border-radius: 8px; text-align: center; box-shadow: 0 1px 3px var(--border); }
.stat-value { font-size: 2rem; font-weight: bold; }
.stat-label { font-size: 0.875rem; opacity: 0.7; }
.chart-container { background: var(--card); padding: 1rem; border-radius: 8px; margin-bottom: 2rem; display: flex; gap: 2rem; align-items: center; box-shadow: 0 1px 3px var(--border); }
.pie-chart { width: 150px; height: 150px; border-radius: 50%%; position: relative; }
.legend { display: flex; flex-direction: column; gap: 0.5rem; }
.legend-item { display: flex; align-items: center; gap: 0.5rem; }
.legend-color { width: 16px; height: 16px; border-radius: 4px; }
.service { background: var(--card); border-radius: 8px; margin-bottom: 1rem; overflow: hidden; box-shadow: 0 1px 3px var(--border); }
.service-header { padding: 1rem; background: var(--hover); cursor: pointer; }
.service-header:hover { background: var(--border); }
table { width: 100%%; border-collapse: collapse; }
th, td { padding: 0.75rem 1rem; text-align: left; border-bottom: 1px solid var(--border); }
th { background: var(--hover); font-weight: 600; }
.conf-high { color: var(--green); }
.conf-med { color: var(--yellow); }
.conf-low { color: var(--red); }
.filter-bar { margin-bottom: 1rem; display: flex; gap: 1rem; align-items: center; }
input[type="text"] { padding: 0.5rem 1rem; border-radius: 4px; border: 1px solid var(--border); background: var(--card); color: var(--text); width: 300px; }
.unmapped { background: var(--card); padding: 1rem; border-radius: 8px; margin-top: 2rem; box-shadow: 0 1px 3px var(--border); }
.unmapped ul { list-style: none; display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 1rem; }
.unmapped li { background: var(--hover); padding: 0.25rem 0.75rem; border-radius: 4px; font-family: monospace; font-size: 0.875rem; }
</style>
</head>
<body>
<h1>tgpiler Mapping Report</h1>

<div class="stats">
<div class="stat-card"><div class="stat-value">%d</div><div class="stat-label">Total Methods</div></div>
<div class="stat-card"><div class="stat-value">%d</div><div class="stat-label">Mapped</div></div>
<div class="stat-card"><div class="stat-value">%d</div><div class="stat-label">Unmapped</div></div>
<div class="stat-card"><div class="stat-value" style="color:var(--green)">%d</div><div class="stat-label">High (&gt;80%%)</div></div>
<div class="stat-card"><div class="stat-value" style="color:var(--yellow)">%d</div><div class="stat-label">Medium (50-80%%)</div></div>
<div class="stat-card"><div class="stat-value" style="color:var(--red)">%d</div><div class="stat-label">Low (&lt;50%%)</div></div>
</div>

<div class="chart-container">
<div class="pie-chart" style="background: conic-gradient(var(--green) 0%% %.1f%%, var(--yellow) %.1f%% %.1f%%, var(--red) %.1f%% 100%%);"></div>
<div class="legend">
<div class="legend-item"><div class="legend-color" style="background:var(--green)"></div>High Confidence: %d</div>
<div class="legend-item"><div class="legend-color" style="background:var(--yellow)"></div>Medium Confidence: %d</div>
<div class="legend-item"><div class="legend-color" style="background:var(--red)"></div>Low Confidence: %d</div>
</div>
</div>

<div class="filter-bar">
<input type="text" id="filter" placeholder="Filter by method or procedure name..." onkeyup="filterTable()">
</div>

<h2 style="margin-bottom:1rem">Mappings by Service</h2>
`,
		stats.TotalMethods, stats.MappedMethods, stats.UnmappedMethods,
		stats.HighConfidence, stats.MediumConfidence, stats.LowConfidence,
		float64(stats.HighConfidence)/float64(stats.MappedMethods)*100,
		float64(stats.HighConfidence)/float64(stats.MappedMethods)*100,
		float64(stats.HighConfidence+stats.MediumConfidence)/float64(stats.MappedMethods)*100,
		float64(stats.HighConfidence+stats.MediumConfidence)/float64(stats.MappedMethods)*100,
		stats.HighConfidence, stats.MediumConfidence, stats.LowConfidence)

	for svcName, mappingList := range serviceMethodMappings {
		names := serviceMethodNames[svcName]
		fmt.Fprintf(cfg.stdout, `<div class="service">
<div class="service-header"><strong>%s</strong> (%d methods)</div>
<table>
<thead><tr><th>RPC Method</th><th>Stored Procedure</th><th>Confidence</th><th>Match Reason</th></tr></thead>
<tbody>
`, svcName, len(mappingList))

		for i, mapping := range mappingList {
			confClass := "conf-high"
			if mapping.Confidence < 0.5 {
				confClass = "conf-low"
			} else if mapping.Confidence < 0.8 {
				confClass = "conf-med"
			}
			fmt.Fprintf(cfg.stdout, `<tr><td>%s</td><td><code>%s</code></td><td class="%s">%.0f%%</td><td>%s</td></tr>
`, names[i], mapping.Procedure.Name, confClass, mapping.Confidence*100, mapping.MatchReason)
		}
		fmt.Fprintf(cfg.stdout, "</tbody></table></div>\n")
	}

	if len(unmapped) > 0 {
		fmt.Fprintf(cfg.stdout, `<div class="unmapped">
<h3>Unmapped Procedures (%d)</h3>
<ul>
`, len(unmapped))
		for _, name := range unmapped {
			fmt.Fprintf(cfg.stdout, "<li>%s</li>\n", name)
		}
		fmt.Fprintf(cfg.stdout, "</ul></div>\n")
	}

	fmt.Fprintf(cfg.stdout, `
<script>
function filterTable() {
  const filter = document.getElementById('filter').value.toLowerCase();
  document.querySelectorAll('table tbody tr').forEach(row => {
    const text = row.textContent.toLowerCase();
    row.style.display = text.includes(filter) ? '' : 'none';
  });
}
</script>
</body>
</html>
`)
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
	opts.Dialect = cfg.sqlDialect

	var buf bytes.Buffer
	if cfg.serviceName != "" {
		// Single service - use original method
		if err := gen.GenerateServiceImpl(cfg.serviceName, opts, &buf); err != nil {
			return err
		}
	} else {
		// All services - use consolidated method with single package header
		if err := gen.GenerateAllServicesImpl(opts, &buf); err != nil {
			return err
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
  --receiver <var>      Receiver variable name (default: r, empty for standalone functions)
  --receiver-type <t>   Receiver type (default: *Repository)
  --preserve-go         Don't strip GO batch separators (default: strip them)
  --sequence-mode <m>   Sequence handling: db, uuid, stub (default: db)
  --annotate[=level]    Add code annotations (default level if no value: standard)
                        Levels: none, minimal, standard, verbose
                          minimal  - TODO markers for patterns needing attention
                          standard - TODOs + original SQL comments
                          verbose  - All + type annotations + section markers
  -f, --force           Allow overwriting existing files
  -h, --help            Show help
  -v, --version         Show version

Backend Options (requires --dml):
  --backend <type>      Backend: sql, grpc, mock, inline (default: sql)
  --fallback-backend <type>  Backend for temp tables: sql, mock (default: sql)
  --grpc-client <var>   gRPC client variable name (default: client)
  --grpc-package <path> Import path for generated gRPC package
  --mock-store <var>    Mock store variable name (default: store)

gRPC Mapping Options (requires --dml --backend=grpc):
  --table-service <map> Table-to-service mappings (format: Table:Service,Table:Service)
  --table-client <map>  Table-to-client var mappings (format: Table:clientVar,Table:clientVar)
  --grpc-mappings <map> Procedure-to-method mappings (format: proc:Service.Method,proc:Service.Method)

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

  # gRPC backend with table-to-service mapping
  tgpiler --dml --backend=grpc --grpc-package=catalogpb \
    --table-service="Products:CatalogService,Orders:OrderService" \
    --table-client="Products:catalogClient,Orders:orderClient" \
    input.sql

  # gRPC backend with explicit procedure mapping
  tgpiler --dml --backend=grpc --grpc-package=orderpb \
    --grpc-mappings="usp_ValidateOrder:OrderService.ValidateOrder" \
    input.sql

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