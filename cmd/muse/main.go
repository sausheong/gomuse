// Command muse renders YAML music scores to stereo WAV files, or serves the
// Muse web app with -s.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sausheong/gomuse/internal/muse"
	"github.com/sausheong/gomuse/internal/server"
)

func main() {
	serve := flag.Bool("s", false, "start the Muse web app")
	addr := flag.String("addr", "0.0.0.0:8888", "address the web app listens on")
	data := flag.String("data", "data", "directory the web app stores generated tunes and scores in")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: muse <score>          render <score>.yaml to <score>.wav")
		fmt.Fprintln(os.Stderr, "       muse -s [flags]       start the web app")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *serve {
		startServer(*addr, *data)
		return
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	arg := flag.Arg(0)
	t1 := time.Now()
	var s muse.Score
	name, err := muse.ParseFile(&s, arg)
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
