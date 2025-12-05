package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	// Show help if no input specified
	if inputFile == "" && *inputDir == "" && !*readStdin {
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
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
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
		dmlConfig := transpiler.DMLConfig{
			Backend:        transpiler.BackendSQL,
			SQLDialect:     cfg.sqlDialect,
			StoreVar:       cfg.storeVar,
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



func printUsage(w io.Writer) {
	fmt.Fprint(w, `tgpiler - T-SQL to Go transpiler

Usage:
  tgpiler [options] <input.sql>
  tgpiler [options] -s < input.sql
  tgpiler [options] -d <path>

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

  # SPLogger with slog (default)
  tgpiler --dml --splogger input.sql

  # SPLogger with database logging
  tgpiler --dml --splogger --logger-type=db --logger-table=ErrorLog input.sql

  # SPLogger with file logging
  tgpiler --dml --splogger --logger-type=file --logger-file=/var/log/sp.json input.sql

  # Generate init() code for SPLogger setup
  tgpiler --dml --splogger --logger-init input.sql

  # Directory processing
  tgpiler -d ./sql -O ./go                # directory to directory
  tgpiler -d ./sql -O ./go -f             # with overwrite

Exit codes:
  0  Success
  1  Parse/transpile error
  2  CLI usage error
`)
}
