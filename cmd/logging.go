package main

import (
	"fmt"
	"strings"
)

func logStep(stepNumber int, total int, instruction string, cacheStatus string, duration float64) {
	trimmed := strings.TrimSpace(instruction)
	op := ""
	if fields := strings.Fields(trimmed); len(fields) > 0 {
		op = strings.ToUpper(fields[0])
	}

	// Cache logging is only allowed for COPY and RUN.
	if op != "COPY" && op != "RUN" {
		fmt.Printf("Step %d/%d : %s\n", stepNumber, total, trimmed)
		return
	}

	if strings.TrimSpace(cacheStatus) == "" {
		fmt.Printf("Step %d/%d : %s\n", stepNumber, total, trimmed)
		return
	}

	fmt.Printf("Step %d/%d : %s [%s] %.2fs\n", stepNumber, total, trimmed, strings.ToUpper(cacheStatus), duration)
}

func logSuccess(digest string, tag string, totalTime float64) {
	_ = totalTime
	fmt.Printf("Successfully built %s %s\n", digest, tag)
}
