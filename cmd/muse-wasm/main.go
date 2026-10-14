//go:build js && wasm

// muse-wasm exposes the filesystem-free engine to a dedicated browser worker.
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/sausheong/gomuse/internal/muse"
	"math"
	"syscall/js"
)

var stream *muse.PCMStream
var callbacks []js.Func

func info(v muse.PlayerInfo) js.Value {
	b, _ := json.Marshal(v)
	return js.Global().Get("JSON").Call("parse", string(b))
}
func pcm(v []int) js.Value {
	b := make([]byte, len(v)*4)
	for i, n := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(float32(n)/32768))
	}
	a := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(a, b)
	return a
}
func expose(name string, fn func([]js.Value) any) {
	cb := js.FuncOf(func(_ js.Value, args []js.Value) (out any) {
		defer func() {
			if r := recover(); r != nil {
				out = map[string]any{"error": fmt.Sprint(r)}
			}
		}()
		return fn(args)
	})
	callbacks = append(callbacks, cb)
	js.Global().Set(name, cb)
}
func main() {
	expose("museOpen", func(args []js.Value) any {
		var err error
		stream, err = muse.NewPCMStream([]byte(args[0].String()))
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return map[string]any{"info": info(stream.Info)}
	})
	expose("museNext", func(args []js.Value) any {
		if stream == nil {
			return map[string]any{"error": "no score loaded"}
		}
		data, done, err := stream.Next(args[0].Int())
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return map[string]any{"pcm": pcm(data), "done": done, "duration": stream.Info.Duration}
	})
	expose("museRender", func(args []js.Value) any {
		i, data, err := muse.RenderPCM([]byte(args[0].String()))
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return map[string]any{"pcm": pcm(data), "info": info(i)}
	})
	select {}
}
