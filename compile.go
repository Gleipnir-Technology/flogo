package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func buildProject() {
	uiMutex.Lock()
	isCompiling = true
	uiMutex.Unlock()

	log.Println("Building project...")

	cmd := exec.Command("go", "build", ".")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	uiMutex.Lock()
	isCompiling = false
	lastBuildOutput = outputStr
	lastBuildSuccess = (err == nil)
	uiMutex.Unlock()

	if err != nil {
		log.Println("Build failed:")
		log.Println(outputStr)
	} else {
		log.Println("Build succeeded!")
	}
}

func determineBuildOutputName() (string, error) {
	// Check if we're in a Go module
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return "", fmt.Errorf("no go.mod file found, not in a Go module")
	}

	// Approach 1: Use go list to get the module name
	cmd := exec.Command("go", "list", "-m")
	output, err := cmd.Output()
	if err == nil {
		// Extract the last part of the module path which is typically the project name
		modulePath := strings.TrimSpace(string(output))
		parts := strings.Split(modulePath, "/")
		return parts[len(parts)-1], nil
	}

	// Approach 2: Read go.mod directly
	data, err := os.ReadFile("go.mod")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
				parts := strings.Split(modulePath, "/")
				return parts[len(parts)-1], nil
			}
		}
	}

	// Approach 3: Fall back to directory name
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}
	return filepath.Base(dir), nil
}
