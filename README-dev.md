# Developer Readme

## Commands
```bash
# init go module
go mod init github.com/adegoodyer/repo-scanner && go mod tidy

# get packages
go get github.com/spf13/cobra
go get github.com/fatih/color
go get gopkg.in/yaml.v3

# run
go run main.go

# install locally
go install ./cmd/repo-scanner
```