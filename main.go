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
var h string

func init() {
	serverFlag = flag.Bool("s", false, "start the Muse service")
	flag.Parse()
	var err error
	dir, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	h = dir + "/static/html/"
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
	t, _ := template.ParseFiles(h+"index.html", h+"try.html", h+"links.html")
	t.Execute(w, nil)
}

func sample(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(h+"sample.html", h+"try.html", h+"links.html")
	vars := mux.Vars(r)
	score, _ := ioutil.ReadFile(dir + "/scores/" + vars["name"] + ".yaml")
	t.Execute(w, string(score))
}

func create(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(h+"tune.html", h+"try.html", h+"links.html")
	r.ParseForm()
	score := r.PostFormValue("score")
	guid := xid.New()
	name, err := parseScore("static/tunes/"+guid.String(), []byte(score))
	var message string
	if err != nil {
		message = err.Error()
	}
	err = ioutil.WriteFile(dir+"/static/scores/"+guid.String()+".yaml", []byte(score), 0644)
	if err != nil {
		message = err.Error()
	}
	data := struct {
		ID       string
		Message  string
		Name     string
		Score    string
		Filename string
	}{guid.String(), message, name, score, "static/tunes/" + guid.String() + ".wav"}
	t.Execute(w, &data)
}

func share(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(h+"share.html", h+"links.html")
	vars := mux.Vars(r)
	score, _ := ioutil.ReadFile(dir + "/static/scores/" + vars["id"] + ".yaml")
	var s *Score
	err := yaml.Unmarshal(score, &s)
	if err != nil {
		log.Println("Cannot unmarshal score file - %v", err)
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

func parseScore(outfile string, score []byte) (name string, err error) {
	var s Score
	name, err = Parse(&s, score, outfile)
	if err != nil {
		log.Println("Cannot parse score file - %v", err)
	}
	return
}

func parseScoreFile(file string) string {
	var s Score
	id, err := ParseFile(&s, file)
	if err != nil {
		log.Println("Cannot parse score file - %v", err)
	}
	return id
}
