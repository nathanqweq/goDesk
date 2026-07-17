package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"godesk/internal/app"
	"godesk/internal/config"
	"godesk/internal/metrics"
	"godesk/internal/server"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			runServe()
			return
		case "--monitoring":
			runMonitoring()
			return
		}
	}

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

func runServe() {
	opts := config.ServiceConfigFromEnv()

	if err := config.SetupLogger(opts.LogFile); err != nil {
		log.Printf("WARN: falha ao configurar log file (%s): %v\n", opts.LogFile, err)
	}

	if err := server.Run(opts); err != nil {
		log.Fatalln(err)
	}
}

func runMonitoring() {
	snap, err := metrics.Read(config.MetricsFileFromEnv())
	if err != nil {
		fmt.Fprintln(os.Stderr, "godesk --monitoring: "+err.Error())
		os.Exit(1)
	}

	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "godesk --monitoring: "+err.Error())
		os.Exit(1)
	}

	fmt.Println(string(b))
}
