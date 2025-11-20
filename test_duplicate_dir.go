package main

import (
	"fmt"
	"path/filepath"
)

// Simulate the duplicate directory prevention logic
func shouldCreateDirectory(directory, pathName string) bool {
	return filepath.Base(directory) != pathName
}

func main() {
	testCases := []struct {
		directory string
		pathName  string
		expected  bool
	}{
		{"/downloads", "David Guetta", true},           // Different names, create
		{"/downloads/David Guetta", "David Guetta", false}, // Same name, skip
		{"/downloads/My Playlist", "David Guetta", true},   // Different names, create
		{"/downloads/David Guetta", "Bebe Rexha", true},    // Different names, create
		{"/downloads", "My Playlist", true},                // Different names, create
	}

	fmt.Println("Testing duplicate directory prevention:")
	fmt.Println("=========================================")

	for _, tc := range testCases {
		result := shouldCreateDirectory(tc.directory, tc.pathName)
		status := "CREATE"
		if !result {
			status = "SKIP"
		}

		expectedStatus := "CREATE"
		if !tc.expected {
			expectedStatus = "SKIP"
		}

		fmt.Printf("Directory: %-25s Path: %-15s -> %s (expected %s)\n",
			fmt.Sprintf("'%s'", tc.directory),
			fmt.Sprintf("'%s'", tc.pathName),
			status,
			expectedStatus)
	}

	fmt.Println("\nExample scenarios:")
	fmt.Println("==================")

	// Scenario 1: Playlist named differently than artist
	fmt.Println("Playlist 'House Music' with David Guetta tracks:")
	fmt.Println("  Current dir: '/downloads/House Music'")
	fmt.Printf("  Path name: 'David Guetta' -> %s\n",
		map[bool]string{true: "CREATE", false: "SKIP"}[shouldCreateDirectory("/downloads/House Music", "David Guetta")])
	fmt.Println("  Result: '/downloads/House Music/David Guetta/filename' ✓")

	// Scenario 2: Playlist named same as artist
	fmt.Println("\nPlaylist 'David Guetta' with David Guetta tracks:")
	fmt.Println("  Current dir: '/downloads/David Guetta'")
	fmt.Printf("  Path name: 'David Guetta' -> %s\n",
		map[bool]string{true: "CREATE", false: "SKIP"}[shouldCreateDirectory("/downloads/David Guetta", "David Guetta")])
	fmt.Println("  Result: '/downloads/David Guetta/filename' ✓ (no duplicate)")
}
