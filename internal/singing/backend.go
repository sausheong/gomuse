// Package singing runs an optional local neural renderer. It is only wired into
// the CLI: YAML scores cannot choose executables or enable server-side commands.
package singing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-audio/wav"
	"github.com/sausheong/gomuse/internal/muse"
)

type Config struct {
	Command        []string `json:"command"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	CacheDir       string   `json:"cache_dir"`
	// Revision identifies backend code, model and vocoder revisions. Change it
	// after updating any of them; otherwise deliberately cached audio is reused.
	Revision string `json:"revision"`
}

type Backend struct {
	cfg Config
	dir string
}

func Load(path string) (*Backend, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var c Config
	dec := json.NewDecoder(io.LimitReader(f, 1<<20))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("singer config: %w", err)
	}
	var trailing any
	if err = dec.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("singer config must contain one JSON object")
	}
	if len(c.Command) == 0 || c.Command[0] == "" {
		return nil, fmt.Errorf("singer config needs a command array")
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 300
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 3600 {
		return nil, fmt.Errorf("singer timeout must be 1..3600 seconds")
	}
	if c.CacheDir != "" && c.Revision == "" {
		return nil, fmt.Errorf("singer cache requires a revision identifying model and backend code")
	}
	dir := filepath.Dir(abs)
	if c.CacheDir != "" && !filepath.IsAbs(c.CacheDir) {
		c.CacheDir = filepath.Join(dir, c.CacheDir)
	}
	if !filepath.IsAbs(c.Command[0]) {
		c.Command[0] = filepath.Join(dir, c.Command[0])
	}
	return &Backend{c, dir}, nil
}

// cappedLog drains all output but keeps only a bounded prefix.
type cappedLog struct{ bytes.Buffer }

func (b *cappedLog) Write(p []byte) (int, error) {
	n := len(p)
	if left := 65536 - b.Len(); left > 0 {
		b.Buffer.Write(p[:min(left, n)])
	}
	return n, nil
}

func (b *Backend) Render(ctx context.Context, r muse.VocalRequest) ([]float64, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	cfg, _ := json.Marshal(b.cfg)
	h := sha256.New()
	h.Write(cfg)
	h.Write(data)
	key := hex.EncodeToString(h.Sum(nil))
	cached := ""
	if b.cfg.CacheDir != "" {
		cached = filepath.Join(b.cfg.CacheDir, key+".wav")
		if _, err = os.Stat(cached); err == nil {
			return readAudio(cached, r)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	dir, err := os.MkdirTemp("", "muse-singing-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	input, output := filepath.Join(dir, "request.json"), filepath.Join(dir, "vocal.wav")
	if err = os.WriteFile(input, data, 0600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(b.cfg.TimeoutSeconds)*time.Second)
	defer cancel()
	args := append([]string{}, b.cfg.Command[1:]...)
	args = append(args, "--request", input, "--output", output)
	cmd := exec.CommandContext(ctx, b.cfg.Command[0], args...)
	cmd.Dir = b.dir
	cmd.WaitDelay = 2 * time.Second
	var log cappedLog
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("renderer stopped: %w", ctx.Err())
		}
		return nil, fmt.Errorf("renderer failed: %w\n%s", err, log.String())
	}
	audio, err := readAudio(output, r)
	if err != nil {
		return nil, err
	}
	if cached != "" {
		if err = os.MkdirAll(b.cfg.CacheDir, 0700); err != nil {
			return nil, err
		}
		raw, err := os.ReadFile(output)
		if err != nil {
			return nil, err
		}
		tmp, err := os.CreateTemp(b.cfg.CacheDir, ".vocal-*.wav")
		if err != nil {
			return nil, err
		}
		defer os.Remove(tmp.Name())
		_, err = tmp.Write(raw)
		closeErr := tmp.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if err = os.Rename(tmp.Name(), cached); err != nil {
			return nil, err
		}
	}
	return audio, nil
}

func readAudio(path string, r muse.VocalRequest) ([]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("renderer did not produce a WAV: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	frames := int(r.Duration*float64(r.SampleRate) + 0.5)
	if st.Size() > int64(frames)*2+65536 {
		return nil, fmt.Errorf("renderer WAV exceeds expected size")
	}
	d := wav.NewDecoder(f)
	d.ReadInfo()
	if d.Err() != nil {
		return nil, fmt.Errorf("invalid renderer WAV: %w", d.Err())
	}
	if d.SampleRate != uint32(r.SampleRate) || d.NumChans != 1 || d.BitDepth != 16 || d.WavAudioFormat != 1 {
		return nil, fmt.Errorf("renderer must return mono 16-bit PCM at %d Hz", r.SampleRate)
	}
	buf, err := d.FullPCMBuffer()
	if err != nil {
		return nil, err
	}
	if len(buf.Data) != frames {
		return nil, fmt.Errorf("renderer returned %d frames, expected %d", len(buf.Data), frames)
	}
	out := make([]float64, len(buf.Data))
	for i, v := range buf.Data {
		out[i] = float64(v) / 32768
	}
	return out, nil
}
