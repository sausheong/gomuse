package singing

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/sausheong/gomuse/internal/muse"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("MUSE_SINGING_HELPER") != "1" {
		return
	}
	for i, a := range os.Args {
		if a == "--output" {
			if os.Getenv("MUSE_SINGING_SLEEP") == "1" {
				time.Sleep(20 * time.Second)
			}
			f, err := os.Create(os.Args[i+1])
			if err != nil {
				os.Exit(2)
			}
			enc := wav.NewEncoder(f, 44100, 16, 1, 1)
			samples := make([]int, 4410)
			for j := range samples {
				samples[j] = 1234
			}
			if err = enc.Write(&audio.IntBuffer{Data: samples, Format: &audio.Format{SampleRate: 44100, NumChannels: 1}, SourceBitDepth: 16}); err != nil {
				os.Exit(2)
			}
			enc.Close()
			f.Close()
			os.Exit(0)
		}
	}
	os.Exit(2)
}
func testBackend(t *testing.T) *Backend {
	t.Helper()
	t.Setenv("MUSE_SINGING_HELPER", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &Backend{cfg: Config{Command: []string{exe, "-test.run=^TestHelperProcess$", "--"}, TimeoutSeconds: 3, CacheDir: filepath.Join(t.TempDir(), "cache"), Revision: "test-v1"}, dir: t.TempDir()}
}
func TestBackendRoundTripAndCache(t *testing.T) {
	b := testBackend(t)
	req := muse.VocalRequest{Version: 1, SampleRate: 44100, Duration: 0.1}
	x, err := b.Render(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(x) != 4410 || x[0] != 1234.0/32768 {
		t.Fatal("wrong decoded audio")
	}
	t.Setenv("MUSE_SINGING_SLEEP", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err = b.Render(ctx, req); err != nil {
		t.Fatalf("cache not used: %v", err)
	}
}
func TestBackendCancellation(t *testing.T) {
	b := testBackend(t)
	t.Setenv("MUSE_SINGING_SLEEP", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := b.Render(ctx, muse.VocalRequest{Version: 1, SampleRate: 44100, Duration: 0.1})
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("got %v", err)
	}
}
func TestConfigRejectsUnknownFields(t *testing.T) {
	p := filepath.Join(t.TempDir(), "singer.json")
	os.WriteFile(p, []byte(`{"command":["python"],"typo":true}`), 0600)
	if _, err := Load(p); err == nil {
		t.Fatal("accepted typo")
	}
}
func TestConfigResolvesExecutableAndCache(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "singer.json")
	data, _ := json.Marshal(Config{Command: []string{"venv/bin/python", "backend.py"}, CacheDir: "cache", Revision: "v1"})
	os.WriteFile(p, data, 0600)
	b, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if b.cfg.Command[0] != filepath.Join(dir, "venv/bin/python") || b.cfg.CacheDir != filepath.Join(dir, "cache") {
		t.Fatal("paths not resolved")
	}
}
