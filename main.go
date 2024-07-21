package main

import (
	"github.com/Roshick/manifest-maestro/internal/wiring"
	"os"
)

func main() {
	os.Exit(wiring.NewApplication().Run())
}
