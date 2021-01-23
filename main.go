package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/xid"
	"gopkg.in/yaml.v2"
)

var serverFlag *bool
var dir string

func init() {
	serverFlag = flag.Bool("s", false, "start the Muse service")
	flag.Parse()
	var err error
	dir, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	if *serverFlag {
		r := mux.NewRouter()
		r.HandleFunc("/", index)
		r.HandleFunc("/sample/{name}", sample)
		r.HandleFunc("/create", create)
		r.HandleFunc("/share/{id}", share)

		r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(dir+"/static"))))

		srv := &http.Server{
			Handler:      r,
			Addr:         "0.0.0.0:8888",
			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
		}
		fmt.Println("Starting Muse server at", srv.Addr)
		log.Fatal(srv.ListenAndServe())

	} else {
		if len(os.Args) > 1 {
			arg := os.Args[1]
			t1 := time.Now()
			parseScoreFile(arg)
			dur := time.Since(t1)
			fmt.Println("Created", arg+".wav", "in", dur.String())
		} else {
			fmt.Println("No score provided")
		}
	}
}

// front page
func index(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(dir + "/static/html/index.html")
	t.Execute(w, nil)
}

func sample(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(dir + "/static/html/sample.html")
	vars := mux.Vars(r)
	score, _ := ioutil.ReadFile(dir + "/scores/" + vars["name"] + ".yaml")
	t.Execute(w, string(score))
}

func create(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(dir + "/static/html/tune.html")
	r.ParseForm()
	score := r.PostFormValue("score")
	guid := xid.New()
	name := parseScore("static/tunes/"+guid.String(), []byte(score))
	data := struct {
		ID       string
		Name     string
		Score    string
		Filename string
	}{guid.String(), name, score, "static/tunes/" + guid.String() + ".wav"}
	err := ioutil.WriteFile(dir+"/static/scores/"+guid.String()+".yaml", []byte(score), 0644)
	if err != nil {
		log.Fatal(err)
	}
	t.Execute(w, &data)
}

func share(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(dir + "/static/html/share.html")
	vars := mux.Vars(r)
	score, _ := ioutil.ReadFile(dir + "/static/scores/" + vars["id"] + ".yaml")
	var s *Score
	err := yaml.Unmarshal(score, &s)
	if err != nil {
		log.Fatalf("Cannot unmarshal score file - %v", err)
		return
	}

	data := struct {
		ID       string
		Name     string
		Score    string
		Filename string
	}{vars["id"], s.Name, string(score), "/static/tunes/" + vars["id"] + ".wav"}
	t.Execute(w, &data)
}

func parseScore(outfile string, score []byte) string {
	var s Score

	id, err := Parse(&s, score, outfile)
	if err != nil {
		log.Fatalf("Cannot parse score file - %v", err)
	}
	return id
}

func parseScoreFile(file string) string {
	var s Score
	id, err := ParseFile(&s, file)
	if err != nil {
		log.Fatalf("Cannot parse score file - %v", err)
	}
	return id
}
