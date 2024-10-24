# repo-scanner

## Overview
  - Recursively scans directory for Dockerfiles and YAML files
- Extracts container image references using regex
- Checks Docker Hub for updates for each image
- Presents results in formatted and coloured   output

## Key Features
- Concurrent update checking with rate limiting
- Supports both Dockerfile and YAML/YML files
- Color-coded output for better visibility
- Handles Docker Hub public images
- Shows last updated dates
- Identifies available updates

## Commands
```bash
# installation
go get install github.com/adegoodyer/repo-scanner@latest

# usage
repo-scanner

# sample output
Container Image Scan Results:
--------------------------------------------------------------------------------
Image: library/node:16.13.0-alpine
File: ./sample-repo/Dockerfile
Last Updated: 2023-11-15
Update available! Latest tag: 21.6.0-alpine
--------------------------------------------------------------------------------
Image: library/golang:1.19
File: ./sample-repo/backend/Dockerfile
Last Updated: 2024-02-20
Update available! Latest tag: 1.22.1
--------------------------------------------------------------------------------
Image: library/redis:6.2
File: ./sample-repo/k8s/deployment.yaml
Last Updated: 2024-03-10
Update available! Latest tag: 7.2.4
--------------------------------------------------------------------------------
Image: library/nginx:1.21
File: ./sample-repo/k8s/deployment.yaml
Last Updated: 2024-03-15
Update available! Latest tag: 1.25.4
--------------------------------------------------------------------------------
```