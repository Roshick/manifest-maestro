package main

import (
	"errors"
	"github.com/Roshick/manifest-maestro/internal/wiring"
	"github.com/joho/godotenv"
	"io/fs"
	"log"
	"os"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("failed to parse .env file: %s", err.Error())
	}

	os.Exit(wiring.NewApplication().Run())
}
