package playbook

import (
	"github.com/fgouteroux/sshot/pkg/config"
	"github.com/fgouteroux/sshot/pkg/executor"
	"github.com/fgouteroux/sshot/pkg/types"
	"github.com/fgouteroux/sshot/pkg/utils"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

func executeOnHost(host types.Host, tasks []types.Task, captureOutput bool, groupName string) types.HostResult {
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

	fmt.Fprintf(writer, "%s┌─ Host: %s%s%s (%s)\n", utils.Color(utils.ColorCyan), utils.Color(utils.ColorBold), host.Name, utils.Color(utils.ColorReset), displayTarget)
	fmt.Fprintf(writer, "%s│%s\n", utils.Color(utils.ColorCyan), utils.Color(utils.ColorReset))

	exec, err := executor.NewExecutor(host, groupName)
	if err != nil {
		fmt.Fprintf(writer, "%s│%s %s✗ Connection failed:%s %v\n", utils.Color(utils.ColorCyan), utils.Color(utils.ColorReset), utils.Color(utils.ColorRed), utils.Color(utils.ColorReset), err)
		fmt.Fprintf(writer, "%s└─ ✗ Connection Failed%s\n\n", utils.Color(utils.ColorRed), utils.Color(utils.ColorReset))
		return types.HostResult{Host: host, Success: false, Error: err, Output: output.String()}
	}
	defer exec.Close()

	if captureOutput {
		exec.OutputWriter = writer
	}

	// Collect facts if collectors are configured
	if globalConfig, ok := config.Cache.Get(); ok && len(globalConfig.Playbook.Facts.Collectors) > 0 {
		fmt.Fprintf(writer, "%s│%s Gathering system facts...\n", utils.Color(utils.ColorCyan), utils.Color(utils.ColorReset))
		if err := exec.CollectFacts(globalConfig.Playbook.Facts); err != nil {
			if types.ExecOptions.Verbose {
				fmt.Fprintf(writer, "  %s✗%s Facts collection failed: %v\n",
					utils.Color(utils.ColorRed), utils.Color(utils.ColorReset), err)
			}
			return types.HostResult{Host: host, Success: false, Error: err, Output: output.String()}
		}
	}

	for i, task := range tasks {
		taskStart := time.Now()
		fmt.Fprintf(writer, "%s│%s [%d/%d] %s\n", utils.Color(utils.ColorCyan), utils.Color(utils.ColorReset), i+1, len(tasks), task.Name)

		if err := exec.ExecuteTask(task); err != nil {
			taskDuration := time.Since(taskStart)
			log.SetOutput(writer)
			log.Printf("  %s✗%s Task failed after %s: %v\n", utils.Color(utils.ColorRed), utils.Color(utils.ColorReset), utils.FormatDuration(taskDuration), err)
			log.SetOutput(os.Stderr)
			fmt.Fprintf(writer, "%s└─ ✗ Failed%s (total time: %s)\n\n", utils.Color(utils.ColorRed), utils.Color(utils.ColorReset), utils.FormatDuration(time.Since(hostStart)))
			return types.HostResult{Host: host, Success: false, Error: err, Output: output.String()}
		}

		taskDuration := time.Since(taskStart)
		if types.ExecOptions.Verbose || taskDuration > 1*time.Second {
			fmt.Fprintf(writer, "%s│%s         %s⏱%s  Task took %s%s%s\n",
				utils.Color(utils.ColorCyan), utils.Color(utils.ColorReset), utils.Color(utils.ColorGray), utils.Color(utils.ColorReset), utils.Color(utils.ColorCyan), utils.FormatDuration(taskDuration), utils.Color(utils.ColorReset))
		}
	}

	totalDuration := time.Since(hostStart)
	fmt.Fprintf(writer, "%s└─ ✓ Completed%s (total time: %s%s%s)\n\n",
		utils.Color(utils.ColorGreen), utils.Color(utils.ColorReset), utils.Color(utils.ColorCyan), utils.FormatDuration(totalDuration), utils.Color(utils.ColorReset))
	return types.HostResult{Host: host, Success: true, Error: nil, Output: output.String()}
}

func executeHostsParallel(hosts []types.Host, tasks []types.Task, groupName string) []types.HostResult {
	var wg sync.WaitGroup
	resultsChan := make(chan types.HostResult, len(hosts))

	for _, host := range hosts {
		wg.Add(1)
		go func(h types.Host) {
			defer wg.Done()
			result := executeOnHost(h, tasks, true, groupName)
			resultsChan <- result
		}(host)
	}

	wg.Wait()
	close(resultsChan)

	var results []types.HostResult
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

func executeHostsSequential(hosts []types.Host, tasks []types.Task, groupName string) []types.HostResult {
	var results []types.HostResult

	for _, host := range hosts {
		result := executeOnHost(host, tasks, false, groupName)
		results = append(results, result)

		if !result.Success {
			break
		}
	}

	return results
}

func executeWithGroups(cfg types.Config) ([]types.HostResult, error) {
	// Store the cfg in the cache if not already set
	config.Cache.Set(&cfg)

	groups := cfg.Inventory.Groups

	sortedGroups := make([]types.Group, len(groups))
	copy(sortedGroups, groups)

	for i := 0; i < len(sortedGroups); i++ {
		for j := i + 1; j < len(sortedGroups); j++ {
			if sortedGroups[j].Order < sortedGroups[i].Order {
				sortedGroups[i], sortedGroups[j] = sortedGroups[j], sortedGroups[i]
			}
		}
	}

	if types.ExecOptions.Verbose {
		log.Printf("[VERBOSE] Executing %d groups in order", len(sortedGroups))
		for _, g := range sortedGroups {
			log.Printf("[VERBOSE]   Group: %s (order: %d, hosts: %d, parallel: %v)",
				g.Name, g.Order, len(g.Hosts), g.Parallel)
		}
	}

	var allResults []types.HostResult
	completedGroups := make(map[string]bool)

	for _, group := range sortedGroups {
		if len(group.DependsOn) > 0 {
			for _, dep := range group.DependsOn {
				if !completedGroups[dep] {
					return allResults, fmt.Errorf("group '%s' depends on '%s' which has not completed", group.Name, dep)
				}
			}
		}

		fmt.Printf("\n%s═══ Group: %s%s%s (order: %d) ═══%s\n", utils.Color(utils.ColorMagenta), utils.Color(utils.ColorBold), group.Name, utils.Color(utils.ColorReset), group.Order, utils.Color(utils.ColorReset))
		if len(group.DependsOn) > 0 {
			fmt.Printf("    %sDependencies:%s %v\n", utils.Color(utils.ColorReset), utils.Color(utils.ColorReset), group.DependsOn)
		}
		fmt.Printf("\n")

		var groupResults []types.HostResult

		if group.Parallel {
			groupResults = executeHostsParallel(group.Hosts, cfg.Playbook.Tasks, group.Name)
		} else {
			groupResults = executeHostsSequential(group.Hosts, cfg.Playbook.Tasks, group.Name)
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

func printPlaybookSummary(results []types.HostResult, totalDuration time.Duration, err error) {
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
		fmt.Printf("║    Total time: %-47s ║\n", utils.FormatDuration(totalDuration))
		fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")

		if types.ExecOptions.Verbose {
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
		if types.ExecOptions.DryRun {
			fmt.Printf("║  ✓ DRY-RUN COMPLETED                                           ║\n")
		} else {
			fmt.Printf("║  ✓ PLAYBOOK COMPLETED SUCCESSFULLY                             ║\n")
		}
		fmt.Printf("║    All %d host(s) completed successfully                        ║\n", successCount)
		fmt.Printf("║    Total time: %-47s ║\n", utils.FormatDuration(totalDuration))
		fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")
	}
}

func Run(playbookPath string, options *types.ExecutionOptions) error {
	types.ExecOptions = *options
	playbookStart := time.Now()

	// Reset the run_once tracking
	types.RunOnceTasks.Lock()
	types.RunOnceTasks.Executed = make(map[string]bool)
	types.RunOnceTasks.Unlock()

	// Load config (either separate or combined files)
	cfg, err := config.Load(playbookPath, types.ExecOptions.InventoryFile)
	if err != nil {
		return err
	}

	// Store the cfg in the cache for global access
	config.Cache.Set(cfg)

	// Apply SSH defaults to hosts
	config.ApplySSHDefaults(cfg)

	parallel := cfg.Playbook.Parallel

	if types.ExecOptions.Verbose {
		log.Printf("[VERBOSE] Playbook: %s", cfg.Playbook.Name)
		log.Printf("[VERBOSE] Execution mode: %s", map[bool]string{true: "parallel", false: "sequential"}[parallel])
		log.Printf("[VERBOSE] Dry-run: %v", types.ExecOptions.DryRun)
	}

	if types.ExecOptions.DryRun {
		fmt.Printf("\n🔍 DRY-RUN MODE - No actual changes will be made\n")
	}

	fmt.Printf("\n╔════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  PLAYBOOK: %-52s║\n", cfg.Playbook.Name)
	if parallel {
		fmt.Printf("║  MODE: Parallel Execution                                      ║\n")
	}
	fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")

	var results []types.HostResult

	if len(cfg.Inventory.Groups) > 0 {
		results, err = executeWithGroups(*cfg)
		if err != nil {
			printPlaybookSummary(results, time.Since(playbookStart), err)
			return fmt.Errorf("playbook execution failed")
		}
	} else if len(cfg.Inventory.Hosts) > 0 {
		if parallel {
			results = executeHostsParallel(cfg.Inventory.Hosts, cfg.Playbook.Tasks, "")
		} else {
			results = executeHostsSequential(cfg.Inventory.Hosts, cfg.Playbook.Tasks, "")
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
