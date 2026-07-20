package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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
		case "--once-per-day":
			runOncePerDay()
			return
		}
	}

	cfg, err := config.FromArgs(os.Args)
	if err != nil {
		fail(err)
	}

	if err := config.SetupLogger(cfg.LogFile); err != nil {
		log.Printf("WARN: falha ao configurar log file (%s): %v\n", cfg.LogFile, err)
	}

	if err := app.Run(cfg); err != nil {
		log.Printf("[app] ERRO: %v\n", err)
		fail(err)
	}
}

// fail imprime uma mensagem curta em stderr — sempre visível pro Zabbix no
// Action log (Reports → Action log), independente de o log detalhado
// (log.Printf acima) ter ido pro arquivo configurado via SetupLogger ou
// ainda estar em stderr — e sai com código != 0, marcando a action como
// falha. Só é chamado no caminho fatal (RAWDATA inválido, YAML de
// política ilegível, ou falha ao consultar/criar o chamado no TopDesk);
// falhas de efeito colateral (email, ack, send_more_info) continuam só
// logadas, sem afetar o exit code.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "godesk: "+err.Error())
	os.Exit(1)
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

// runOncePerDay consulta o godesk serve rodando na mesma máquina (é lá que
// a lista "um por dia" vive, só em memória — ver internal/app/onceperday.go
// e internal/server/onceperday.go) e imprime as entradas de hoje. Usado
// pra conferir se a deduplicação de e-mail está funcionando.
func runOncePerDay() {
	opts := config.ServiceConfigFromEnv()

	url := "http://" + opts.ListenAddr + "/once-per-day"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "godesk --once-per-day: "+err.Error())
		os.Exit(1)
	}
	if opts.Token != "" {
		req.Header.Set("Authorization", "Bearer "+opts.Token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "godesk --once-per-day: falha ao consultar "+url+": "+err.Error())
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "godesk --once-per-day: servidor respondeu %d: %s\n", resp.StatusCode, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(body))
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
