package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tgpiler", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		inputDir     = fs.String("d", "", "Read all .sql files from directory")
		inputDirL    = fs.String("dir", "", "Read all .sql files from directory")
		output       = fs.String("o", "", "Write to single output file")
		outputL      = fs.String("output", "", "Write to single output file")
		outDir       = fs.String("O", "", "Write to output directory (creates if needed)")
		outDirL      = fs.String("outdir", "", "Write to output directory (creates if needed)")
		force        = fs.Bool("f", false, "Allow overwriting existing files")
		forceL       = fs.Bool("force", false, "Allow overwriting existing files")
		packageName  = fs.String("p", "main", "Package name for generated code")
		packageNameL = fs.String("pkg", "main", "Package name for generated code")
		showHelp     = fs.Bool("h", false, "Show help")
		helpL        = fs.Bool("help", false, "Show help")
		showVer      = fs.Bool("v", false, "Show version")
		versionL     = fs.Bool("version", false, "Show version")
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

	// Validate flag combinations
	if err := validateFlags(inputFile, *inputDir, *output, *outDir); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	// Execute based on mode
	cfg := &config{
		inputFile:   inputFile,
		inputDir:    *inputDir,
		output:      *output,
		outDir:      *outDir,
		force:       *force,
		packageName: *packageName,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
	}

	if err := execute(cfg); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

type config struct {
	inputFile   string
	inputDir    string
	output      string
	outDir      string
	force       bool
	packageName string
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
}

func validateFlags(inputFile, inputDir, output, outDir string) error {
	// Check for conflicting input modes
	if inputFile != "" && inputDir != "" {
		return fmt.Errorf("cannot specify both input file and --dir")
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
	default:
		return executeStdin(cfg)
	}
}

func executeStdin(cfg *config) error {
	source, err := io.ReadAll(cfg.stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	result, err := transpiler.Transpile(string(source), cfg.packageName)
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

	result, err := transpiler.Transpile(string(source), cfg.packageName)
	if err != nil {
		return fmt.Errorf("%s: %w", cfg.inputFile, err)
	}

	return writeOutput(cfg, cfg.inputFile, result)
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

		result, err := transpiler.Transpile(string(source), cfg.packageName)
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
  tgpiler [options] [input.sql]

Input (mutually exclusive):
  (no argument)         Read from stdin
  <file.sql>            Read single file
  -d, --dir <path>      Read all .sql files from directory

Output (mutually exclusive):
  (no flag)             Write to stdout
  -o, --output <file>   Write to single file
  -O, --outdir <path>   Write to directory (creates if needed)

Options:
  -p, --pkg <name>      Package name for generated code (default: main)
  -f, --force           Allow overwriting existing files
  -h, --help            Show help
  -v, --version         Show version

Examples:
  tgpiler < input.sql                    # stdin to stdout
  tgpiler input.sql                      # file to stdout
  tgpiler input.sql -o output.go         # file to file
  tgpiler -d ./sql -O ./go               # directory to directory
  tgpiler -d ./sql -O ./go -f            # with overwrite
  tgpiler -p mypackage input.sql         # custom package name

Exit codes:
  0  Success
  1  Parse/transpile error
  2  CLI usage error
`)
}
