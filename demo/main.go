package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("Hello from Docksmith container!")
	d, _ := os.Getwd()
	fmt.Printf("Working directory: %s\n", d)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "KEY=") || strings.HasPrefix(e, "APP_") {
			fmt.Printf("ENV: %s\n", e)
		}
	}
}
