package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type ImageInfo struct {
	Name         string
	Tag          string
	FilePath     string
	LatestTag    string
	UpdateNeeded bool
	CheckError   error
	LastUpdated  string
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "imgscan [directory]",
		Short: "Scan repository for container images and check for updates",
		Args:  cobra.MaximumNArgs(1),
		Run:   runScan,
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runScan(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	images := scanDirectory(dir)
	if len(images) == 0 {
		fmt.Println("No container images found")
		return
	}

	checkUpdates(images)
	printResults(images)
}

func scanDirectory(root string) []ImageInfo {
	var images []ImageInfo
	imageRegex := regexp.MustCompile(`(?:FROM|image:)\s+([a-zA-Z0-9\-\./_]+)(?::([a-zA-Z0-9\-\.\_]+))?`)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if d.IsDir() || (!strings.HasSuffix(d.Name(), "Dockerfile") &&
			!strings.HasSuffix(d.Name(), ".yaml") &&
			!strings.HasSuffix(d.Name(), ".yml")) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			matches := imageRegex.FindStringSubmatch(scanner.Text())
			if len(matches) > 1 {
				tag := "latest"
				if len(matches) > 2 && matches[2] != "" {
					tag = matches[2]
				}
				images = append(images, ImageInfo{
					Name:     matches[1],
					Tag:      tag,
					FilePath: path,
				})
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
	}

	return images
}

func checkUpdates(images []ImageInfo) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit concurrent requests

	for i := range images {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			checkImageUpdate(&images[idx])
		}(i)
	}

	wg.Wait()
}

func checkImageUpdate(img *ImageInfo) {
	// For Docker Hub public images
	if !strings.Contains(img.Name, "/") {
		img.Name = "library/" + img.Name
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags", img.Name)
	resp, err := http.Get(url)
	if err != nil {
		img.CheckError = err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		img.CheckError = fmt.Errorf("failed to fetch image info: %s", resp.Status)
		return
	}

	var result struct {
		Results []struct {
			Name        string    `json:"name"`
			LastUpdated time.Time `json:"last_updated"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		img.CheckError = err
		return
	}

	for _, tag := range result.Results {
		if tag.Name == "latest" {
			img.LastUpdated = tag.LastUpdated.Format("2006-01-02")
			if img.Tag == "latest" {
				img.UpdateNeeded = true
			}
			break
		}
		if tag.Name == img.Tag {
			img.LastUpdated = tag.LastUpdated.Format("2006-01-02")
			break
		}
	}

	if len(result.Results) > 0 {
		img.LatestTag = result.Results[0].Name
		if img.Tag != img.LatestTag {
			img.UpdateNeeded = true
		}
	}
}

func printResults(images []ImageInfo) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	bold.Println("\nContainer Image Scan Results:")
	fmt.Println(strings.Repeat("-", 80))

	for _, img := range images {
		bold.Printf("Image: %s:%s\n", img.Name, img.Tag)
		fmt.Printf("File: %s\n", img.FilePath)

		if img.CheckError != nil {
			red.Printf("Error checking updates: %v\n", img.CheckError)
		} else {
			fmt.Printf("Last Updated: %s\n", img.LastUpdated)
			if img.UpdateNeeded {
				yellow.Printf("Update available! Latest tag: %s\n", img.LatestTag)
			} else {
				green.Println("Up to date")
			}
		}
		fmt.Println(strings.Repeat("-", 80))
	}
}
