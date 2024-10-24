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
	"gopkg.in/yaml.v3"
)

type ImageInfo struct {
	Name         string
	Tag          string
	FilePath     string
	Resource     string // Kubernetes resource type
	ResourceName string // Kubernetes resource name
	Container    string // Container name within the resource
	LatestTag    string
	UpdateNeeded bool
	CheckError   error
	LastUpdated  string
}

type KubeResource struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec interface{} `yaml:"spec"`
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "imgscan [directory]",
		Short: "Scan repository for container images and check for updates",
		Args:  cobra.MaximumNArgs(1),
		Run:   runScan,
	}

	rootCmd.PersistentFlags().BoolP("kubernetes-only", "k", false, "Only scan Kubernetes manifests")
	rootCmd.PersistentFlags().BoolP("show-summary", "s", false, "Show summary statistics")

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

	kubernetesOnly, _ := cmd.Flags().GetBool("kubernetes-only")
	showSummary, _ := cmd.Flags().GetBool("show-summary")

	images := scanDirectory(dir, kubernetesOnly)
	if len(images) == 0 {
		fmt.Println("No container images found")
		return
	}

	checkUpdates(images)
	printResults(images, showSummary)
}

func scanDirectory(root string, kubernetesOnly bool) []ImageInfo {
	var images []ImageInfo
	imageRegex := regexp.MustCompile(`(?:FROM|image:)\s+([a-zA-Z0-9\-\./_]+)(?::([a-zA-Z0-9\-\.\_]+))?`)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		isKube := strings.HasSuffix(d.Name(), ".yaml") || strings.HasSuffix(d.Name(), ".yml")
		isDockerfile := strings.HasSuffix(d.Name(), "Dockerfile")

		if !isKube && !isDockerfile {
			return nil
		}

		if kubernetesOnly && !isKube {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if isKube {
			// Split YAML documents
			docs := strings.Split(string(content), "---")
			for _, doc := range docs {
				if strings.TrimSpace(doc) == "" {
					continue
				}

				var resource KubeResource
				if err := yaml.Unmarshal([]byte(doc), &resource); err != nil {
					continue
				}

				// Skip non-workload resources
				if !isKubernetesWorkload(resource.Kind) {
					continue
				}

				images = append(images, extractKubernetesImages(doc, path, resource)...)
			}
		} else {
			// Handle Dockerfile
			scanner := bufio.NewScanner(strings.NewReader(string(content)))
			for scanner.Scan() {
				matches := imageRegex.FindStringSubmatch(scanner.Text())
				if len(matches) > 1 {
					tag := "latest"
					if len(matches) > 2 && matches[2] != "" {
						tag = matches[2]
					}
					images = append(images, ImageInfo{
						Name:         matches[1],
						Tag:          tag,
						FilePath:     path,
						Resource:     "Dockerfile",
						ResourceName: filepath.Base(filepath.Dir(path)),
					})
				}
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
	}

	return images
}

func isKubernetesWorkload(kind string) bool {
	workloadKinds := map[string]bool{
		"Deployment":            true,
		"StatefulSet":           true,
		"DaemonSet":             true,
		"Job":                   true,
		"CronJob":               true,
		"Pod":                   true,
		"ReplicaSet":            true,
		"ReplicationController": true,
	}
	return workloadKinds[kind]
}

func extractKubernetesImages(doc, path string, resource KubeResource) []ImageInfo {
	var images []ImageInfo

	var containers []map[string]interface{}

	// Extract spec based on resource kind
	switch resource.Kind {
	case "CronJob":
		if spec, ok := resource.Spec.(map[string]interface{}); ok {
			if jobTemplate, ok := spec["jobTemplate"].(map[string]interface{}); ok {
				if jobSpec, ok := jobTemplate["spec"].(map[string]interface{}); ok {
					if template, ok := jobSpec["template"].(map[string]interface{}); ok {
						if podSpec, ok := template["spec"].(map[string]interface{}); ok {
							containers = extractContainers(podSpec)
						}
					}
				}
			}
		}
	default:
		if spec, ok := resource.Spec.(map[string]interface{}); ok {
			if template, ok := spec["template"].(map[string]interface{}); ok {
				if podSpec, ok := template["spec"].(map[string]interface{}); ok {
					containers = extractContainers(podSpec)
				}
			}
		}
	}

	// Process containers
	for _, container := range containers {
		if image, ok := container["image"].(string); ok {
			name, tag := parseImageString(image)
			images = append(images, ImageInfo{
				Name:         name,
				Tag:          tag,
				FilePath:     path,
				Resource:     resource.Kind,
				ResourceName: resource.Metadata.Name,
				Container:    container["name"].(string),
			})
		}
	}

	return images
}

func extractContainers(podSpec map[string]interface{}) []map[string]interface{} {
	var containers []map[string]interface{}

	// Regular containers
	if regularContainers, ok := podSpec["containers"].([]interface{}); ok {
		for _, c := range regularContainers {
			if container, ok := c.(map[string]interface{}); ok {
				containers = append(containers, container)
			}
		}
	}

	// Init containers
	if initContainers, ok := podSpec["initContainers"].([]interface{}); ok {
		for _, c := range initContainers {
			if container, ok := c.(map[string]interface{}); ok {
				containers = append(containers, container)
			}
		}
	}

	return containers
}

func parseImageString(image string) (name, tag string) {
	parts := strings.Split(image, ":")
	name = parts[0]
	tag = "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}
	return
}

func checkUpdates(images []ImageInfo) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5)

	for i := range images {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			checkImageUpdate(&images[idx])
		}(i)
	}

	wg.Wait()
}

func checkImageUpdate(img *ImageInfo) {
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

func printResults(images []ImageInfo, showSummary bool) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	bold.Println("\nContainer Image Scan Results:")
	fmt.Println(strings.Repeat("-", 80))

	// Group images by file
	imagesByFile := make(map[string][]ImageInfo)
	for _, img := range images {
		imagesByFile[img.FilePath] = append(imagesByFile[img.FilePath], img)
	}

	// Print results grouped by file
	for filePath, fileImages := range imagesByFile {
		bold.Printf("File: %s\n", filePath)
		fmt.Println(strings.Repeat("-", 40))

		for _, img := range fileImages {
			if img.Resource != "Dockerfile" {
				bold.Printf("%s: %s\n", img.Resource, img.ResourceName)
				fmt.Printf("Container: %s\n", img.Container)
			}

			fmt.Printf("Image: %s:%s\n", img.Name, img.Tag)

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
			fmt.Println(strings.Repeat("-", 40))
		}
		fmt.Println()
	}

	if showSummary {
		printSummary(images)
	}
}

func printSummary(images []ImageInfo) {
	bold := color.New(color.Bold)

	var totalImages, needUpdate, errors int
	resourceCounts := make(map[string]int)

	for _, img := range images {
		totalImages++
		if img.UpdateNeeded {
			needUpdate++
		}
		if img.CheckError != nil {
			errors++
		}
		resourceCounts[img.Resource]++
	}

	bold.Println("\nScan Summary:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Total images scanned: %d\n", totalImages)
	fmt.Printf("Images needing updates: %d\n", needUpdate)
	fmt.Printf("Errors encountered: %d\n", errors)

	bold.Println("\nResources Found:")
	for resource, count := range resourceCounts {
		fmt.Printf("%s: %d\n", resource, count)
	}
}
