package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
)

// maxSamplesPerChannel caps how many samples per channel the web app will
// synthesise, so a huge score can't be used to run the server out of memory.
// The command line tool has no such limit.
const maxSamplesPerChannel = 6000000

// maxRequestBody caps the size of a submitted score, again to keep the
// server's memory use bounded.
const maxRequestBody = 256 * 1024 // 256 KiB

// samplePattern is the strict set of characters allowed in a /sample/{name}
// or score file name.
var samplePattern = regexp.MustCompile(`^[a-z0-9._-]+$`)

// baseDir is the absolute directory that static/ (and scores/, for the
// command line tool) live under. htmlDir is derived from it once the
// server starts.
var (
	baseDir string
	htmlDir string
)

// editSecret signs the per-tune edit tokens handed out as cookies (see
// editToken below), so it's generated fresh for each server run.
var editSecret []byte

// the four page templates, parsed once at startup
var (
	indexTmpl  *template.Template
	sampleTmpl *template.Template
	tuneTmpl   *template.Template
	shareTmpl  *template.Template
)

func main() {
	serverFlag := flag.Bool("s", false, "start the Muse service")
	flag.Parse()

	if *serverFlag {
		startServer()
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("No score provided")
		os.Exit(1)
	}

	arg := os.Args[1]
	t1 := time.Now()
	var s Score
	name, err := ParseFile(&s, arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse score file - %v\n", err)
		os.Exit(1)
	}
	dur := time.Since(t1)
	fmt.Println("Created tune", name, "in", arg+".wav", "in", dur.String())
}

// startServer sets up the directories, templates and routes, then serves.
func startServer() {
	router := setupServer(resolveBaseDir())

	srv := &http.Server{
		Handler:      router,
		Addr:         "0.0.0.0:8888",
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}
	fmt.Println("Starting Muse server at", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

// setupServer wires up base, templates and routes under the given base
// directory and returns the resulting handler. Split out from startServer
// so tests can point it at a temporary directory instead of a real listener.
func setupServer(base string) http.Handler {
	baseDir = base
	htmlDir = filepath.Join(baseDir, "static", "html")

	editSecret = make([]byte, 32)
	if _, err := rand.Read(editSecret); err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(baseDir, "static", "tunes"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "static", "scores"), 0755); err != nil {
		log.Fatal(err)
	}

	parseTemplates()

	router := http.NewServeMux()
	router.HandleFunc("GET /{$}", index)
	router.HandleFunc("GET /sample/{name}", sample)
	router.HandleFunc("POST /create", create)
	router.HandleFunc("GET /share/{id}", share)
	router.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(baseDir, "static")))))
	return router
}

// resolveBaseDir finds the directory that holds static/. It prefers the
// directory the executable lives in (the normal installed layout), but
// falls back to the current working directory - `go run . -s` builds the
// binary into a temp dir, so the executable's dir won't have static/ in it.
func resolveBaseDir() string {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if info, err := os.Stat(filepath.Join(exeDir, "static")); err == nil && info.IsDir() {
			return exeDir
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return cwd
}

// parseTemplates parses every page once at startup. template.Must panics
// (and so fails fast at boot) if any template is broken.
func parseTemplates() {
	indexTmpl = template.Must(template.ParseFiles(
		filepath.Join(htmlDir, "index.html"),
		filepath.Join(htmlDir, "try.html"),
		filepath.Join(htmlDir, "links.html"),
	))
	sampleTmpl = template.Must(template.ParseFiles(
		filepath.Join(htmlDir, "sample.html"),
		filepath.Join(htmlDir, "try.html"),
		filepath.Join(htmlDir, "links.html"),
	))
	tuneTmpl = template.Must(template.ParseFiles(
		filepath.Join(htmlDir, "tune.html"),
		filepath.Join(htmlDir, "try.html"),
		filepath.Join(htmlDir, "links.html"),
	))
	shareTmpl = template.Must(template.ParseFiles(
		filepath.Join(htmlDir, "share.html"),
		filepath.Join(htmlDir, "links.html"),
	))
}

// front page
func index(w http.ResponseWriter, r *http.Request) {
	if err := indexTmpl.Execute(w, nil); err != nil {
		log.Printf("cannot render index page - %v", err)
	}
}

// show a sample score
func sample(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !samplePattern.MatchString(name) || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	score, err := os.ReadFile(filepath.Join(baseDir, "scores", name+".yaml"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := sampleTmpl.Execute(w, string(score)); err != nil {
		log.Printf("cannot render sample page - %v", err)
	}
}

// editToken returns the per-tune token that proves the caller is the one
// this server handed a tune id to, rather than someone who merely knows (or
// has guessed) that id from its public share link. The share ids from
// github.com/rs/xid are sequential/predictable, so the id itself is not
// enough to authorise overwriting an existing tune.
func editToken(id string) string {
	mac := hmac.New(sha256.New, editSecret)
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

// setEditCookie gives the caller the token for the tune it just created or
// re-created, scoped to /create only, so it's never sent to the public
// /share or /static routes.
func setEditCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "edit_" + id,
		Value:    editToken(id),
		Path:     "/create",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ownsTune reports whether the request carries the edit cookie this server
// handed out for id, i.e. whether the caller was actually given this id
// rather than guessing it.
func ownsTune(r *http.Request, id string) bool {
	c, err := r.Cookie("edit_" + id)
	return err == nil && hmac.Equal([]byte(c.Value), []byte(editToken(id)))
}

// create the wav file given the score
func create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// get the score
	score := r.PostFormValue("score")

	// id must either be blank (a new tune), or a valid, existing-format xid
	// whose edit cookie the caller holds. A syntactically valid id the
	// caller doesn't hold the cookie for is most likely a guessed public
	// share id rather than one this server gave out, so it's never reused -
	// a fresh id is minted instead, which means the tune it names can never
	// be overwritten this way.
	id := r.PostFormValue("guid")
	switch {
	case id == "":
		id = xid.New().String()
	default:
		if _, err := xid.FromString(id); err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if !ownsTune(r, id) {
			id = xid.New().String()
		}
	}
	setEditCookie(w, id)

	// error message if something goes wrong
	var message string

	// parse the score into a wave file
	var s Score
	name, err := Parse(&s, []byte(score), filepath.Join(baseDir, "static", "tunes", id), maxSamplesPerChannel)
	if err != nil {
		// a failure writing the wav file (disk full, permissions, ...)
		// wraps an *fs.PathError that names the server's absolute install
		// path - show the visitor a generic message instead and log the
		// real one. Parse's own validation errors (bad note, unknown
		// envelope, tune too long, ...) never wrap a filesystem error, so
		// those are still shown verbatim.
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			log.Printf("cannot create wav file for %s - %v", id, err)
			message = "could not create the tune, please try again"
		} else {
			message = err.Error()
		}
	}

	// write the score to a score file for sharing later
	if err := os.WriteFile(filepath.Join(baseDir, "static", "scores", id+".yaml"), []byte(score), 0644); err != nil {
		log.Printf("cannot write score file for %s - %v", id, err)
		if message == "" {
			message = "could not save the score, please try again"
		}
	}

	// anonymous struct to send back to the page
	data := struct {
		ID       string
		Message  string
		Name     string
		Score    string
		Filename string
	}{id, message, name, score, "static/tunes/" + id + ".wav"}

	if err := tuneTmpl.Execute(w, &data); err != nil {
		log.Printf("cannot render tune page - %v", err)
	}
}

// share page for sharing to Facebook, Twitter etc
func share(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := xid.FromString(id); err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	// read from the score file, given the id
	score, err := os.ReadFile(filepath.Join(baseDir, "static", "scores", id+".yaml"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// unmarshal the score file into a Score struct
	var s Score
	if err := yaml.Unmarshal(score, &s); err != nil {
		log.Printf("cannot unmarshal score file - %v", err)
		http.Error(w, "cannot read score", http.StatusInternalServerError)
		return
	}

	// anonymous struct to send back to the page
	data := struct {
		ID       string
		Name     string
		Score    string
		Filename string
	}{id, s.Name, string(score), "/static/tunes/" + id + ".wav"}

	if err := shareTmpl.Execute(w, &data); err != nil {
		log.Printf("cannot render share page - %v", err)
	}
}
