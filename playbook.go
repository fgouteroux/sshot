package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

func executeOnHost(host Host, tasks []Task, captureOutput bool, groupName string) HostResult {
	var output bytes.Buffer
	var writer io.Writer = os.Stdout

	if captureOutput {
		writer = &output
	}

	displayTarget := host.Address
	if displayTarget == "" {
		displayTarget = host.Hostname
	}

	hostStart := time.Now()

	fmt.Fprintf(writer, "%s┌─ Host: %s%s%s (%s)\n", color(ColorCyan), color(ColorBold), host.Name, color(ColorReset), displayTarget)
	fmt.Fprintf(writer, "%s│%s\n", color(ColorCyan), color(ColorReset))

	executor, err := NewExecutor(host, groupName)
	if err != nil {
		fmt.Fprintf(writer, "%s│%s %s✗ Connection failed:%s %v\n", color(ColorCyan), color(ColorReset), color(ColorRed), color(ColorReset), err)
		fmt.Fprintf(writer, "%s└─ ✗ Connection Failed%s\n\n", color(ColorRed), color(ColorReset))
		return HostResult{Host: host, Success: false, Error: err, Output: output.String()}
	}
	defer executor.Close()

	if captureOutput {
		executor.outputWriter = writer
	}

	// Collect facts if collectors are configured
	if globalConfig, ok := configCache.Get(); ok && len(globalConfig.Playbook.Facts.Collectors) > 0 {
		fmt.Fprintf(writer, "%s│%s Gathering system facts...\n", color(ColorCyan), color(ColorReset))
		if err := executor.CollectFacts(globalConfig.Playbook.Facts); err != nil {
			if execOptions.Verbose {
				fmt.Fprintf(writer, "  %s✗%s Facts collection failed: %v\n",
					color(ColorRed), color(ColorReset), err)
			}
			return HostResult{Host: host, Success: false, Error: err, Output: output.String()}
		}
	}

	for i, task := range tasks {
		taskStart := time.Now()
		fmt.Fprintf(writer, "%s│%s [%d/%d] %s\n", color(ColorCyan), color(ColorReset), i+1, len(tasks), task.Name)

		if err := executor.ExecuteTask(task); err != nil {
			taskDuration := time.Since(taskStart)
			log.SetOutput(writer)
			log.Printf("  %s✗%s Task failed after %s: %v\n", color(ColorRed), color(ColorReset), formatDuration(taskDuration), err)
			log.SetOutput(os.Stderr)
			fmt.Fprintf(writer, "%s└─ ✗ Failed%s (total time: %s)\n\n", color(ColorRed), color(ColorReset), formatDuration(time.Since(hostStart)))
			return HostResult{Host: host, Success: false, Error: err, Output: output.String()}
		}

		taskDuration := time.Since(taskStart)
		if execOptions.Verbose || taskDuration > 1*time.Second {
			fmt.Fprintf(writer, "%s│%s         %s⏱%s  Task took %s%s%s\n",
				color(ColorCyan), color(ColorReset), color(ColorGray), color(ColorReset), color(ColorCyan), formatDuration(taskDuration), color(ColorReset))
		}
	}

	totalDuration := time.Since(hostStart)
	fmt.Fprintf(writer, "%s└─ ✓ Completed%s (total time: %s%s%s)\n\n",
		color(ColorGreen), color(ColorReset), color(ColorCyan), formatDuration(totalDuration), color(ColorReset))
	return HostResult{Host: host, Success: true, Error: nil, Output: output.String()}
}

func executeHostsParallel(hosts []Host, tasks []Task, groupName string) []HostResult {
	var wg sync.WaitGroup
	resultsChan := make(chan HostResult, len(hosts))

	for _, host := range hosts {
		wg.Add(1)
		go func(h Host) {
			defer wg.Done()
			result := executeOnHost(h, tasks, true, groupName)
			resultsChan <- result
		}(host)
	}

	wg.Wait()
	close(resultsChan)

	var results []HostResult
	for result := range resultsChan {
		results = append(results, result)
	}

	for _, host := range hosts {
		for _, result := range results {
			if result.Host.Name == host.Name {
				fmt.Print(result.Output)
				break
			}
		}
	}

	return results
}

func executeHostsSequential(hosts []Host, tasks []Task, groupName string) []HostResult {
	var results []HostResult

	for _, host := range hosts {
		result := executeOnHost(host, tasks, false, groupName)
		results = append(results, result)

		if !result.Success {
			break
		}
	}

	return results
}

func executeWithGroups(config Config) ([]HostResult, error) {
	// Store the config in the cache if not already set
	configCache.Set(&config)

	groups := config.Inventory.Groups

	sortedGroups := make([]Group, len(groups))
	copy(sortedGroups, groups)

	for i := 0; i < len(sortedGroups); i++ {
		for j := i + 1; j < len(sortedGroups); j++ {
			if sortedGroups[j].Order < sortedGroups[i].Order {
				sortedGroups[i], sortedGroups[j] = sortedGroups[j], sortedGroups[i]
			}
		}
	}

	if execOptions.Verbose {
		log.Printf("[VERBOSE] Executing %d groups in order", len(sortedGroups))
		for _, g := range sortedGroups {
			log.Printf("[VERBOSE]   Group: %s (order: %d, hosts: %d, parallel: %v)",
				g.Name, g.Order, len(g.Hosts), g.Parallel)
		}
	}

	var allResults []HostResult
	completedGroups := make(map[string]bool)

	for _, group := range sortedGroups {
		if len(group.DependsOn) > 0 {
			for _, dep := range group.DependsOn {
				if !completedGroups[dep] {
					return allResults, fmt.Errorf("group '%s' depends on '%s' which has not completed", group.Name, dep)
				}
			}
		}

		fmt.Printf("\n%s═══ Group: %s%s%s (order: %d) ═══%s\n", color(ColorMagenta), color(ColorBold), group.Name, color(ColorReset), group.Order, color(ColorReset))
		if len(group.DependsOn) > 0 {
			fmt.Printf("    %sDependencies:%s %v\n", color(ColorReset), color(ColorReset), group.DependsOn)
		}
		fmt.Printf("\n")

		var groupResults []HostResult

		if group.Parallel {
			groupResults = executeHostsParallel(group.Hosts, config.Playbook.Tasks, group.Name)
		} else {
			groupResults = executeHostsSequential(group.Hosts, config.Playbook.Tasks, group.Name)
		}

		allResults = append(allResults, groupResults...)

		groupFailed := false
		for _, result := range groupResults {
			if !result.Success {
				groupFailed = true
			}
		}

		if groupFailed {
			return allResults, fmt.Errorf("group '%s' failed", group.Name)
		}

		completedGroups[group.Name] = true
	}

	return allResults, nil
}

func printPlaybookSummary(results []HostResult, totalDuration time.Duration, err error) {
	successCount := 0
	failCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	fmt.Printf("╔════════════════════════════════════════════════════════════════╗\n")
	if err != nil || failCount > 0 {
		fmt.Printf("║  ✗ PLAYBOOK FAILED                                             ║\n")
		fmt.Printf("║    Successful: %-3d  Failed: %-3d                                ║\n", successCount, failCount)
		fmt.Printf("║    Total time: %-47s ║\n", formatDuration(totalDuration))
		fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")

		if execOptions.Verbose {
			if err != nil {
				log.Printf("[VERBOSE] Error: %v", err)
			}
			log.Printf("[VERBOSE] Failed hosts:")
			for _, result := range results {
				if !result.Success {
					log.Printf("[VERBOSE]   - %s: %v", result.Host.Name, result.Error)
				}
			}
		}
	} else {
		if execOptions.DryRun {
			fmt.Printf("║  ✓ DRY-RUN COMPLETED                                           ║\n")
		} else {
			fmt.Printf("║  ✓ PLAYBOOK COMPLETED SUCCESSFULLY                             ║\n")
		}
		fmt.Printf("║    All %d host(s) completed successfully                        ║\n", successCount)
		fmt.Printf("║    Total time: %-47s ║\n", formatDuration(totalDuration))
		fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")
	}
}

func RunPlaybook(playbookPath string) error {
	playbookStart := time.Now()

	// Reset the run_once tracking
	runOnceTasks.Lock()
	runOnceTasks.executed = make(map[string]bool)
	runOnceTasks.Unlock()

	// Load config (either separate or combined files)
	config, err := loadConfig(playbookPath, execOptions.InventoryFile)
	if err != nil {
		return err
	}

	// Store the config in the cache for global access
	configCache.Set(config)

	// Apply SSH defaults to hosts
	applySSHDefaults(config)

	parallel := config.Playbook.Parallel

	if execOptions.Verbose {
		log.Printf("[VERBOSE] Playbook: %s", config.Playbook.Name)
		log.Printf("[VERBOSE] Execution mode: %s", map[bool]string{true: "parallel", false: "sequential"}[parallel])
		log.Printf("[VERBOSE] Dry-run: %v", execOptions.DryRun)
	}

	if execOptions.DryRun {
		fmt.Printf("\n🔍 DRY-RUN MODE - No actual changes will be made\n")
	}

	fmt.Printf("\n╔════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  PLAYBOOK: %-52s║\n", config.Playbook.Name)
	if parallel {
		fmt.Printf("║  MODE: Parallel Execution                                      ║\n")
	}
	fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")

	var results []HostResult

	if len(config.Inventory.Groups) > 0 {
		results, err = executeWithGroups(*config)
		if err != nil {
			printPlaybookSummary(results, time.Since(playbookStart), err)
			return fmt.Errorf("playbook execution failed")
		}
	} else if len(config.Inventory.Hosts) > 0 {
		if parallel {
			results = executeHostsParallel(config.Inventory.Hosts, config.Playbook.Tasks, "")
		} else {
			results = executeHostsSequential(config.Inventory.Hosts, config.Playbook.Tasks, "")
		}
	} else {
		return fmt.Errorf("no hosts or groups defined in inventory")
	}

	// Check if any host failed
	hasFailure := false
	for _, result := range results {
		if !result.Success {
			hasFailure = true
			break
		}
	}

	printPlaybookSummary(results, time.Since(playbookStart), nil)

	if hasFailure {
		return fmt.Errorf("playbook execution failed")
	}

	return nil
}
