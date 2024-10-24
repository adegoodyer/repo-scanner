# repo-scanner

## Overview
- Recursively scans directory for Dockerfiles and YAML files
- Extracts container image references using regex
- Checks Docker Hub for updates for each image
- Presents results in formatted and coloured   output

## Key Features
- Concurrent update checking with rate limiting
- Supports both Dockerfile and YAML/YML files
- Comprehensive Kubernetes support:
  - Deployments, StatefulSets, DaemonSets
  - CronJobs and Jobs
  - Pods and ReplicaSets
  - Init containers support
  - Multiple containers per pod
- Grouped output by file for better readability
- Resource type and name display
- Container name display
- Color-coded output for better visibility
- Handles Docker Hub public images
- Shows last updated dates
- Identifies available updates

## Commands
```bash
# installation
go install github.com/adegoodyer/repo-scanner/cmd/repo-scanner@latest

# usage
repo-scanner
repo-scanner --kubernetes-only (-k) flag to only scan Kubernetes manifests
repo-scanner --show-summary (-s) flag for summary statistics

# sample output
Container Image Scan Results:
--------------------------------------------------------------------------------
File: ./manifests/deployment.yaml
----------------------------------------
Deployment: backend-service
Container: api
Image: library/golang:1.19
Last Updated: 2024-02-20
Update available! Latest tag: 1.22.1
----------------------------------------
Container: sidecar
Image: library/nginx:1.21
Last Updated: 2024-03-15
Update available! Latest tag: 1.25.4
----------------------------------------

File: ./manifests/cronjob.yaml
----------------------------------------
CronJob: backup-job
Container: backup
Image: library/postgres:13
Last Updated: 2024-03-01
Update available! Latest tag: 16.1
----------------------------------------

Scan Summary:
----------------------------------------
Total images scanned: 3
Images needing updates: 3
Errors encountered: 0

Resources Found:
Deployment: 1
CronJob: 1
```