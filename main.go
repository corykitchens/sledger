package main

import (
	"github.com/corykitchens/sledger/cmd"
	_ "github.com/lib/pq"
)

func main() {
	cmd.Execute()
}
