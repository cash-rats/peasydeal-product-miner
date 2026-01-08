package main

import (
	"os"

	"peasydeal-product-miner/cmd/runner/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}

