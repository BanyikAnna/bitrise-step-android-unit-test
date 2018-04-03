package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-tools/go-android/gradle"
	"github.com/bitrise-tools/go-steputils/stepconf"
)

// Configs ...
type Configs struct {
	ProjectLocation   string `env:"project_location,required"`
	ReportPathPattern string `env:"report_path_pattern"`
	Variant           string `env:"variant"`
	Module            string `env:"module"`
}

func failf(f string, args ...interface{}) {
	log.Errorf(f, args...)
	os.Exit(1)
}

func main() {
	var config Configs

	if err := stepconf.Parse(&config); err != nil {
		failf("Couldn't create step config: %v\n", err)
	}

	stepconf.Print(config)

	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")

	log.Printf("- Deploy dir: %s", deployDir)
	fmt.Println()

	gradleProject, err := gradle.NewProject(config.ProjectLocation)
	if err != nil {
		failf("Failed to open project, error: %s", err)
	}

	testTask := gradleProject.
		GetModule(config.Module).
		GetTask("test")

	log.Infof("Variants:")
	fmt.Println()

	variants, err := testTask.GetVariants()
	if err != nil {
		failf("Failed to fetch variants, error: %s", err)
	}

	filteredVariants := variants.Filter(config.Variant)

	for _, variant := range variants {
		if sliceutil.IsStringInSlice(variant, filteredVariants) {
			log.Donef("✓ %s", variant)
		} else {
			log.Printf("- %s", variant)
		}
	}

	fmt.Println()

	if len(filteredVariants) == 0 {
		errMsg := fmt.Sprintf("No variant matching for: (%s)", config.Variant)
		if config.Module != "" {
			errMsg += fmt.Sprintf(" in module: [%s]", config.Module)
		}
		failf(errMsg)
	}

	if config.Variant == "" {
		log.Warnf("No variant specified, test will run on all variants")
		fmt.Println()
	}

	started := time.Now()

	log.Infof("Run test:")
	testErr := testTask.Run(filteredVariants)
	if testErr != nil {
		log.Errorf("Test task failed, error: %v", testErr)
	}
	fmt.Println()

	log.Infof("Export reports:")
	fmt.Println()

	reports, err := gradleProject.FindDirs(started, config.ReportPathPattern, true)
	if err != nil {
		failf("failed to find reports, error: %v", err)
	}

	if len(reports) == 0 {
		log.Warnf("No reports found with pattern: %s", config.ReportPathPattern)
		log.Warnf("If you have changed default report export path in your gradle files then you might need to change ReportPathPattern accordingly.")
		os.Exit(0)
	}

	for _, report := range reports {
		report.Name += ".zip"

		exists, err := pathutil.IsPathExists(filepath.Join(deployDir, report.Name))
		if err != nil {
			failf("failed to check path, error: %v", err)
		}

		artifactName := filepath.Base(report.Path)

		if exists {
			timestamp := time.Now().Format("20060102150405")
			ext := filepath.Ext(report.Name)
			name := strings.TrimSuffix(filepath.Base(report.Name), ext)
			report.Name = fmt.Sprintf("%s-%s%s", name, timestamp, ext)
		}

		log.Printf("  Export [ %s => $BITRISE_DEPLOY_DIR/%s ]", artifactName, report.Name)

		if err := report.ExportZIP(deployDir); err != nil {
			log.Warnf("failed to export report (%s), error: %v", report.Path, err)
			continue
		}
	}

	if testErr != nil {
		os.Exit(1)
	}
}