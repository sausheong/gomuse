package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// newTestServer copies static/html and a scores fixture into a fresh temp
// base dir and wires up a handler against it via setupServer. Handlers use
// package-level state (baseDir, htmlDir, the parsed templates), so these
// tests run sequentially rather than in parallel.
func newTestServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	base := t.TempDir()

	dstHTML := filepath.Join(base, "static", "html")
	if err := os.MkdirAll(dstHTML, 0755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("static/html")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join("static/html", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dstHTML, e.Name()), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	scoresDir := filepath.Join(base, "scores")
	if err := os.MkdirAll(scoresDir, 0755); err != nil {
		t.Fatal(err)
	}
	scale, err := os.ReadFile("scores/scale.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scoresDir, "scale.yaml"), scale, 0644); err != nil {
		t.Fatal(err)
	}

	return setupServer(base), base
}

const validScore = `
name: Test tune
key: C
length: 0.05
envelope: flat
harmonic: first
volume: 100
sections:
  - C1: ["c4"]
    C2: ["c4"]
`

func postCreate(t *testing.T, ts *httptest.Server, form url.Values) *http.Response {
	t.Helper()
	resp, err := http.PostForm(ts.URL+"/create", form)
	if err != nil {
		t.Fatalf("POST /create failed: %v", err)
	}
	return resp
}

// -- index -------------------------------------------------------------

func TestWebIndex(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
}

// -- /sample/{name} -----------------------------------------------------

func TestWebSampleFound(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/sample/scale")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /sample/scale = %d, want 200", resp.StatusCode)
	}
}

func TestWebSampleNotFound(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/sample/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /sample/nope = %d, want 404", resp.StatusCode)
	}
}

func TestWebSampleTraversalRejected(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// "/sample/.." never reaches the sample handler at all: the standard
	// mux cleans the path and redirects to "/" before routing, which is
	// itself the safe outcome (it can never serve scores/../something).
	// The other, encoded forms of traversal do reach the handler and must
	// be rejected by its own validation.
	targets := []string{
		ts.URL + "/sample/%2e%2e",
		ts.URL + "/sample/%2e%2e%2fscale",
		ts.URL + "/sample/UPPER",
		ts.URL + "/sample/with%20space",
	}
	for _, target := range targets {
		resp, err := http.Get(target)
		if err != nil {
			t.Fatalf("GET %s failed: %v", target, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", target, resp.StatusCode)
		}
	}

	resp, err := http.Get(ts.URL + "/sample/..")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/" {
		t.Fatalf("GET /sample/.. ended at %s (%d), want redirected to / (200)", resp.Request.URL.Path, resp.StatusCode)
	}

	// belt and braces: exercise the handler's own validation directly for a
	// name containing ".." that a mux might otherwise clean/route around
	req := httptest.NewRequest(http.MethodGet, "/sample/x", nil)
	req.SetPathValue("name", "../../etc/passwd")
	rec := httptest.NewRecorder()
	sample(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("sample(\"../../etc/passwd\") = %d, want 404", rec.Code)
	}
}

// -- POST /create --------------------------------------------------------

func TestWebCreateValidScore(t *testing.T) {
	handler, base := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp := postCreate(t, ts, url.Values{"score": {validScore}})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /create = %d, want 200; body: %s", resp.StatusCode, body)
	}

	id := extractGUID(t, string(body))
	if id == "" {
		t.Fatalf("could not find guid in response body: %s", body)
	}
	if _, err := os.Stat(filepath.Join(base, "static", "tunes", id+".wav")); err != nil {
		t.Fatalf("expected wav file to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "static", "scores", id+".yaml")); err != nil {
		t.Fatalf("expected score yaml to be written: %v", err)
	}
}

func TestWebCreateBadGUIDRejected(t *testing.T) {
	handler, base := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp := postCreate(t, ts, url.Values{"score": {validScore}, "guid": {"../../evil"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /create with bad guid = %d, want 400", resp.StatusCode)
	}

	// nothing should have been written outside the tunes/scores dirs under base
	if _, err := os.Stat(filepath.Join(base, "static", "tunes", "evil.wav")); err == nil {
		t.Fatal("evil.wav should not have been written")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(base), "evil.yaml")); err == nil {
		t.Fatal("nothing should have been written outside base")
	}
}

// A caller that holds the edit cookie handed out when a tune was first
// created can legitimately re-create (overwrite) that same id - this is the
// "re-create" button on the tune page.
func TestWebCreateGUIDReuseWithCookieOverwrites(t *testing.T) {
	handler, base := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	resp1, err := client.PostForm(ts.URL+"/create", url.Values{"score": {validScore}})
	if err != nil {
		t.Fatal(err)
	}
	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()
	id := extractGUID(t, string(body1))
	if id == "" {
		t.Fatalf("could not find guid in response body: %s", body1)
	}

	edited := strings.Replace(validScore, "name: Test tune", "name: Edited tune", 1)
	resp2, err := client.PostForm(ts.URL+"/create", url.Values{"score": {edited}, "guid": {id}})
	if err != nil {
		t.Fatal(err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("POST /create reusing own guid = %d, want 200; body: %s", resp2.StatusCode, body2)
	}
	if got := extractGUID(t, string(body2)); got != id {
		t.Fatalf("re-creating with the owning cookie should keep id %s, got %s", id, got)
	}
	saved, err := os.ReadFile(filepath.Join(base, "static", "scores", id+".yaml"))
	if err != nil {
		t.Fatalf("cannot read saved score: %v", err)
	}
	if !strings.Contains(string(saved), "Edited tune") {
		t.Fatalf("score file was not overwritten by the owning caller: %s", saved)
	}
}

// A caller that supplies a syntactically valid id it was never given the
// edit cookie for - e.g. one it read off another tune's public share link -
// must not be able to overwrite that tune. A predictable id (github.com/rs/xid
// ids are sequential) is exactly the scenario this guards against.
func TestWebCreateGUIDWithoutCookieCannotOverwrite(t *testing.T) {
	handler, base := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// the victim creates a tune and its id becomes a public share id
	victimResp := postCreate(t, ts, url.Values{"score": {validScore}})
	victimBody, _ := io.ReadAll(victimResp.Body)
	victimResp.Body.Close()
	victimID := extractGUID(t, string(victimBody))
	if victimID == "" {
		t.Fatalf("could not find victim guid in response body: %s", victimBody)
	}
	victimScoreBefore, err := os.ReadFile(filepath.Join(base, "static", "scores", victimID+".yaml"))
	if err != nil {
		t.Fatalf("cannot read victim score: %v", err)
	}

	// an attacker, with no cookie at all, supplies the victim's public id
	defaced := strings.Replace(validScore, "name: Test tune", "name: DEFACED", 1)
	attackerResp := postCreate(t, ts, url.Values{"score": {defaced}, "guid": {victimID}})
	attackerBody, _ := io.ReadAll(attackerResp.Body)
	attackerResp.Body.Close()
	if attackerResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /create with a guessed guid = %d, want 200 (silently given a new id); body: %s", attackerResp.StatusCode, attackerBody)
	}
	attackerID := extractGUID(t, string(attackerBody))
	if attackerID == "" {
		t.Fatalf("could not find guid in attacker response body: %s", attackerBody)
	}
	if attackerID == victimID {
		t.Fatal("attacker without the edit cookie must not be able to reuse the victim's id")
	}

	victimScoreAfter, err := os.ReadFile(filepath.Join(base, "static", "scores", victimID+".yaml"))
	if err != nil {
		t.Fatalf("cannot read victim score after attack: %v", err)
	}
	if string(victimScoreAfter) != string(victimScoreBefore) {
		t.Fatalf("victim's score was overwritten: now %s", victimScoreAfter)
	}
	if strings.Contains(string(victimScoreAfter), "DEFACED") {
		t.Fatal("victim's score was defaced")
	}
}

func TestWebCreateEscapesScriptInName(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	score := `
name: "<script>alert(1)</script>"
key: C
length: 0.05
envelope: flat
harmonic: first
volume: 100
sections:
  - C1: ["c4"]
    C2: ["c4"]
`
	resp := postCreate(t, ts, url.Values{"score": {score}})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /create = %d, want 200", resp.StatusCode)
	}
	if strings.Contains(string(body), "<script>alert(1)</script>") {
		t.Fatalf("response contains unescaped script tag: %s", body)
	}
	if !strings.Contains(string(body), "&lt;script&gt;") {
		t.Fatalf("response does not contain the expected escaped name: %s", body)
	}

	id := extractGUID(t, string(body))
	if id == "" {
		t.Fatalf("could not find guid in response body: %s", body)
	}

	shareResp, err := http.Get(ts.URL + "/share/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer shareResp.Body.Close()
	shareBody, _ := io.ReadAll(shareResp.Body)
	if shareResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /share/%s = %d, want 200; body: %s", id, shareResp.StatusCode, shareBody)
	}
	if strings.Contains(string(shareBody), "<script>alert(1)</script>") {
		t.Fatalf("share page contains unescaped script tag: %s", shareBody)
	}
	if !strings.Contains(string(shareBody), "&lt;script&gt;") {
		t.Fatalf("share page does not contain the expected escaped name: %s", shareBody)
	}
}

func TestWebCreateOversizedBodyRejected(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	huge := strings.Repeat("a", maxRequestBody+1024)
	resp := postCreate(t, ts, url.Values{"score": {huge}})
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("POST /create with oversized body = 200, want rejected")
	}
}

func TestWebCreateTooLongScoreShowsMessageNoCrash(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	score := `
name: Too long
key: C
length: 1.0
envelope: flat
harmonic: first
volume: 100
sections:
  - C1: ["100000000:c4"]
    C2: []
`
	resp := postCreate(t, ts, url.Values{"score": {score}})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /create with too-long score = %d, want 200 (page with error message); body: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "too long") {
		t.Fatalf("response does not show the 'too long' error message: %s", body)
	}
}

// a tiny request body with a single note stacking thousands of pitches into
// one chord must be rejected, not accepted and turned into a multi-gigabyte
// allocation: encode() allocates one sample slice per pitch in the chord, so
// chord width - not just declared duration - has to be bounded.
func TestWebCreateWideChordRejectedNotOOM(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	pitches := make([]string, 10000)
	for i := range pitches {
		pitches[i] = "c4"
	}
	score := `
name: Wide chord
key: C
length: 0.5
envelope: flat
harmonic: first
volume: 100
sections:
  - C1: ["` + strings.Join(pitches, "-") + `"]
    C2: []
`
	resp := postCreate(t, ts, url.Values{"score": {score}})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /create with wide chord = %d, want 200 (page with error message); body: %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "<audio") {
		t.Fatalf("wide chord should have been rejected, not synthesised: %s", body)
	}
}

// when writing the wav file fails (disk full, permissions, ...), the page
// must show a generic message, not the raw OS error - that error wraps an
// *fs.PathError naming the server's absolute install directory.
func TestWebCreateWavWriteFailureHidesPath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root ignores permission bits")
	}
	handler, base := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	tunesDir := filepath.Join(base, "static", "tunes")
	if err := os.Chmod(tunesDir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(tunesDir, 0755)

	resp := postCreate(t, ts, url.Values{"score": {validScore}})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /create with unwritable tunes dir = %d, want 200 (page with error message); body: %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), base) {
		t.Fatalf("response leaks the server's absolute base path: %s", body)
	}
	if strings.Contains(string(body), "permission denied") {
		t.Fatalf("response leaks the raw OS error: %s", body)
	}
	if !strings.Contains(string(body), "could not create the tune") {
		t.Fatalf("response does not show the generic failure message: %s", body)
	}
}

// -- GET /share/{id} ------------------------------------------------------

func TestWebShareUnknownID(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// a well-formed but never-created xid
	resp, err := http.Get(ts.URL + "/share/9m4e2mr0ui3e8a215n4g")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /share/<unknown> = %d, want 404", resp.StatusCode)
	}
}

func TestWebShareInvalidID(t *testing.T) {
	handler, _ := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/share/not-an-xid")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /share/not-an-xid = %d, want 400", resp.StatusCode)
	}
}

// -- helpers ---------------------------------------------------------------

var guidPattern = regexp.MustCompile(`name="guid"\s+value="([^"]+)"`)

func extractGUID(t *testing.T, body string) string {
	t.Helper()
	m := guidPattern.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return m[1]
}
