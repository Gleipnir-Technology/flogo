package main

import (
	"log"
	"os/exec"
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
