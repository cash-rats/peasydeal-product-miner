package main

import (
	"os"

	"peasydeal-product-miner/cmd/devtool/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
