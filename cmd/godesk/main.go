package main

import (
	"log"
	"os"

	"godesk/internal/app"
	"godesk/internal/config"
)

func main() {
	cfg, err := config.FromArgs(os.Args)
	if err != nil {
		log.Fatalln(err)
	}

	if err := config.SetupLogger(cfg.LogFile); err != nil {
		log.Printf("WARN: falha ao configurar log file (%s): %v\n", cfg.LogFile, err)
	}

	if err := app.Run(cfg); err != nil {
		log.Fatalln(err)
	}
}
