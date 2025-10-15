package main

import (
	"flag"
	"log"
	"os"
)

var execOptions ExecutionOptions

func main() {
	dryRun := flag.Bool("dry-run", false, "Run in dry-run mode (don't execute commands)")
	dryRunShort := flag.Bool("n", false, "Run in dry-run mode (shorthand)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	verboseShort := flag.Bool("v", false, "Enable verbose logging (shorthand)")
	progress := flag.Bool("progress", false, "Show progress indicators for long-running tasks")
	noColor := flag.Bool("no-color", false, "Disable colored output")
	fullOutput := flag.Bool("full-output", false, "Show complete command output without truncation")
	fullOutputShort := flag.Bool("f", false, "Show complete command output (shorthand)")
	inventory := flag.String("inventory", "", "Path to inventory file (if separate from playbook)")
	inventoryShort := flag.String("i", "", "Path to inventory file (shorthand)")

	flag.Parse()

	execOptions.DryRun = *dryRun || *dryRunShort
	execOptions.Verbose = *verbose || *verboseShort
	execOptions.Progress = *progress
	execOptions.NoColor = *noColor
	execOptions.FullOutput = *fullOutput || *fullOutputShort

	// Use inventory flag (prefer long form over short form)
	if *inventory != "" {
		execOptions.InventoryFile = *inventory
	} else if *inventoryShort != "" {
		execOptions.InventoryFile = *inventoryShort
	}

	if flag.NArg() < 1 {
		log.Println("Usage: sshot [options] <playbook.yml>")
		log.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	playbookPath := flag.Arg(0)
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		log.Fatalf("Playbook file not found: %s", playbookPath)
	}

	// If inventory file is specified, validate it exists
	if execOptions.InventoryFile != "" {
		if _, err := os.Stat(execOptions.InventoryFile); os.IsNotExist(err) {
			log.Fatalf("Inventory file not found: %s", execOptions.InventoryFile)
		}
	}

	if execOptions.Verbose {
		log.Printf("[VERBOSE] Starting sshot")
		log.Printf("[VERBOSE] Playbook path: %s", playbookPath)
		if execOptions.InventoryFile != "" {
			log.Printf("[VERBOSE] Inventory path: %s", execOptions.InventoryFile)
		}
		log.Printf("[VERBOSE] Options: dry-run=%v, verbose=%v, progress=%v, no-color=%v, full-output=%v",
			execOptions.DryRun, execOptions.Verbose, execOptions.Progress, execOptions.NoColor, execOptions.FullOutput)
	}

	if err := RunPlaybook(playbookPath); err != nil {
		log.Fatalf("Playbook execution failed: %v", err)
	}
}
