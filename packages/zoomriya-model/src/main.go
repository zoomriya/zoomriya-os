// zoomriya-modeld: the box's model service (box-desktop.md §4, Myira). It
// keeps one language model in memory, in llama.cpp's llama-server, and
// answers everyone who needs it on loopback (127.0.0.1:8070): Box Desktop's
// Myira and, when installed, the ZOOMRIYA apps, with the API they already
// use (/myira/api/v1/complete, /complete/stream, /describe, /model).
//
// It has no assistant of its own: no prompts, no tools, no memory. It checks
// who is asking (a ZOOMRIYA ID token), queues the answers one at a time by
// priority, lets an admin choose the model, and forgets every question as soon
// as it is answered. Nothing sent here is stored or logged.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// Config is where everything is: the box's paths by default, the
// environment's elsewhere (development, tests).
type Config struct {
	// Addr: loopback only, never behind the gateway.
	Addr  string
	IDAPI string
	// ModelsDir: the installed GGUF chat models (and their vision files).
	ModelsDir string
	// DefaultModel: used until an admin chooses one.
	DefaultModel string
	// StateDir: the admin's choice (model.json).
	StateDir string
	// Server: llama.cpp's llama-server; ServerAddr, where it listens, on
	// loopback, behind a key only this service knows.
	Server     string
	ServerAddr string
	Context    int
	// GPULayers: how many of the model's layers go to the graphics chip
	// (Vulkan): all of them by default; 0 keeps the model on the CPU. With
	// no graphics chip llama.cpp uses the CPU whatever this says.
	GPULayers int
	// Preload: load the chosen model at start, so the first answer is quick.
	Preload bool
}

func env(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return fallback
}

func configFromEnv() Config {
	context, err := strconv.Atoi(env("ZOOMRIYA_MODEL_CONTEXT", "8192"))
	if err != nil || context < 512 {
		context = 8192
	}
	gpu, err := strconv.Atoi(env("ZOOMRIYA_MODEL_GPU_LAYERS", "99"))
	if err != nil || gpu < 0 {
		gpu = 99
	}
	return Config{
		Addr:         env("ZOOMRIYA_MODEL_ADDR", "127.0.0.1:8070"),
		IDAPI:        env("ZOOMRIYA_ID_URL", "http://127.0.0.1:8050/id/api/v1"),
		ModelsDir:    env("ZOOMRIYA_MODEL_DIR", "/data/models/llms"),
		DefaultModel: env("ZOOMRIYA_MODEL_FILE", "gemma-4-E2B-it-Q4_K_M.gguf"),
		StateDir:     env("ZOOMRIYA_MODEL_STATE", "/var/lib/zoomriya-model"),
		Server:       env("ZOOMRIYA_MODEL_SERVER", "/usr/lib/zoomriya-model/llama-server"),
		ServerAddr:   env("ZOOMRIYA_MODEL_SERVER_ADDR", "127.0.0.1:8071"),
		Context:      context,
		GPULayers:    gpu,
		Preload:      env("ZOOMRIYA_MODEL_PRELOAD", "true") != "false",
	}
}

func main() {
	log.SetFlags(0)
	cfg := configFromEnv()
	engine := NewEngine(cfg, startLlamaServer(cfg))
	auth := NewAuth(cfg.IDAPI + "/auth/jwks.json")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.Preload {
		if file := engine.SelectedFile(); fileExists(filepath.Join(cfg.ModelsDir, file)) {
			if err := engine.StartLoading(file); err != nil {
				log.Printf("the model didn't start loading: %v", err)
			}
		} else {
			log.Printf("no model to load yet: %s isn't in %s", file, cfg.ModelsDir)
		}
	}

	// A model downloaded or copied later (box-desktop.md BD-17) is loaded as
	// soon as it's all there.
	go engine.WatchForModel(ctx, 10*time.Second)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           NewAPI(engine, auth),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("zoomriya-model on %s", cfg.Addr)
	err := server.ListenAndServe()
	engine.Close()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
