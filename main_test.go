package main

import (
	"testing"
)

func TestExecutionOptions_Defaults(t *testing.T) {
	opts := ExecutionOptions{}

	if opts.DryRun {
		t.Error("DryRun should default to false")
	}
	if opts.Verbose {
		t.Error("Verbose should default to false")
	}
	if opts.Progress {
		t.Error("Progress should default to false")
	}
	if opts.NoColor {
		t.Error("NoColor should default to false")
	}
}

func TestExecutionOptions_Settings(t *testing.T) {
	// Save original values
	originalDryRun := execOptions.DryRun
	originalVerbose := execOptions.Verbose
	originalProgress := execOptions.Progress
	originalNoColor := execOptions.NoColor

	defer func() {
		execOptions.DryRun = originalDryRun
		execOptions.Verbose = originalVerbose
		execOptions.Progress = originalProgress
		execOptions.NoColor = originalNoColor
	}()

	// Test setting values
	execOptions.DryRun = true
	execOptions.Verbose = true
	execOptions.Progress = true
	execOptions.NoColor = true

	if !execOptions.DryRun {
		t.Error("DryRun should be true")
	}
	if !execOptions.Verbose {
		t.Error("Verbose should be true")
	}
	if !execOptions.Progress {
		t.Error("Progress should be true")
	}
	if !execOptions.NoColor {
		t.Error("NoColor should be true")
	}
}
