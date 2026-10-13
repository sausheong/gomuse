// Command muse renders YAML music scores to stereo WAV files, or serves the
// Muse web app with -s.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/sausheong/gomuse/internal/muse"
	"github.com/sausheong/gomuse/internal/server"
	"github.com/sausheong/gomuse/internal/singing"
)

func main() {
	expand := flag.String("expand", "", "write expanded score YAML without rendering audio")
	serve := flag.Bool("s", false, "start the Muse web app")
	addr := flag.String("addr", "0.0.0.0:8888", "address the web app listens on")
	data := flag.String("data", "data", "directory the web app stores generated tunes and scores in")
	singerConfig := flag.String("singer", "", "local neural singer JSON configuration (CLI only)")
	singingPlan := flag.String("singing-plan", "", "write vocal timeline JSON without loading a model or rendering audio")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: muse <score>          render <score>.yaml to <score>.wav")
		fmt.Fprintln(os.Stderr, "       muse -s [flags]       start the web app")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *serve {
		if *singerConfig != "" || *singingPlan != "" || *expand != "" {
			log.Fatal("expansion and singing options are supported in CLI mode only")
		}
		startServer(*addr, *data)
		return
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	arg := strings.TrimSuffix(flag.Arg(0), ".yaml")
	if *expand != "" {
		if *singingPlan != "" || *singerConfig != "" {
			log.Fatal("-expand cannot be combined with singing options")
		}
		doc, err := os.ReadFile(arg + ".yaml")
		if err != nil {
			log.Fatal(err)
		}
		expanded, err := muse.ExpandScore(doc)
		if err != nil {
			log.Fatal(err)
		}
		// Exclusive creation prevents accidentally replacing the editable source.
		f, err := os.OpenFile(*expand, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			log.Fatal(err)
		}
		_, err = f.Write(expanded)
		closeErr := f.Close()
		if err != nil {
			log.Fatal(err)
		}
		if closeErr != nil {
			log.Fatal(closeErr)
		}
		fmt.Println("Wrote expanded score to", *expand)
		return
	}
	if *singingPlan != "" {
		doc, err := os.ReadFile(arg + ".yaml")
		if err != nil {
			log.Fatal(err)
		}
		plan, err := muse.SingingPlan(doc)
		if err != nil {
			log.Fatal(err)
		}
		if err = os.WriteFile(*singingPlan, append(plan, '\n'), 0600); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Wrote vocal timeline to", *singingPlan)
		return
	}
	t1 := time.Now()
	var s muse.Score
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var backend muse.Singer
	if *singerConfig != "" {
		b, err := singing.Load(*singerConfig)
		if err != nil {
			log.Fatal(err)
		}
		backend = b
	}
	score, err := os.ReadFile(arg + ".yaml")
	if err != nil {
		log.Fatal(err)
	}
	name, err := muse.ParseWithSinger(ctx, &s, score, arg, 0, backend)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse score file - %v\n", err)
		os.Exit(1)
	}
	dur := time.Since(t1)
	fmt.Println("Created tune", name, "in", arg+".wav", "in", dur.String())
}

// startServer sets up the web app and serves it until it fails.
func startServer(addr, dataDir string) {
	app, err := server.New(dataDir)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Handler:      app.Handler(),
		Addr:         addr,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}
	fmt.Println("Starting Muse server at", srv.Addr, "-", app)
	log.Fatal(srv.ListenAndServe())
}
