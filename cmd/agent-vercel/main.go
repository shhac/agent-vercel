package main

import (
	"github.com/shhac/agent-vercel/internal/cli"
	libcli "github.com/shhac/lib-agent-cli/cli"
)

var version = "dev"

func main() {
	libcli.Run(cli.NewRootCmd(version))
}
