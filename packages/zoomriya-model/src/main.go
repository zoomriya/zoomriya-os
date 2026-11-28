// zoomriya-modeld: the box's model service (box-desktop.md §4, Myira). It
// keeps one language model in memory, in llama.cpp's llama-server, and
// answers everyone who needs it on its socket (/run/zoomriya/model.sock, no
// TCP port): Box Desktop's Myira and, when installed, the ZOOMRIYA apps, with
// the API they already use (/myira/api/v1/complete, /complete/stream,
// /describe, /model).
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

	"zoomriya.com/os/localhttp"
)

// Config is where everything is: the box's paths by default, the
// environment's elsewhere (development, tests).
type Config struct {
	// Socket: where it answers when systemd passes no socket
	// (zoomriya-model.socket does on the box): development. Never behind the
	// gateway.
	Socket string
	// IDAPI: ZOOMRIYA ID's address (unix:id.sock:/id/api/v1), for its keys.
	IDAPI string
	// ModelsDir: the installed GGUF chat models (and their vision files).
	ModelsDir string
	// DefaultModel: used until an admin chooses one.
	DefaultModel string
	// StateDir: the admin's choice (model.json).
	StateDir string
	// Server: llama.cpp's llama-server; ServerSocket, the socket it listens
	// on, in this service's own runtime folder, behind a key only this
	// service knows.
	Server       string
	ServerSocket string
	Context      int
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
		Socket:       env("ZOOMRIYA_MODEL_SOCKET", localhttp.SocketPath("model.sock")),
		IDAPI:        env("ZOOMRIYA_ID_URL", "unix:id.sock:/id/api/v1"),
		ModelsDir:    env("ZOOMRIYA_MODEL_DIR", "/data/models/llms"),
		DefaultModel: env("ZOOMRIYA_MODEL_FILE", "gemma-4-E2B-it-Q4_K_M.gguf"),
		StateDir:     env("ZOOMRIYA_MODEL_STATE", "/var/lib/zoomriya-model"),
		Server:       env("ZOOMRIYA_MODEL_SERVER", "/usr/lib/zoomriya-model/llama-server"),
		ServerSocket: env("ZOOMRIYA_MODEL_SERVER_SOCKET", "/run/zoomriya-model/llama.sock"),
		Context:      context,
		GPULayers:    gpu,
		Preload:      env("ZOOMRIYA_MODEL_PRELOAD", "true") != "false",
	}
}

func main() {
	log.SetFlags(0)
	cfg := configFromEnv()
	engine := NewEngine(cfg, startLlamaServer(cfg))
	idBase, idClient, err := localhttp.Client(cfg.IDAPI, 5*time.Second)
	if err != nil {
		log.Fatalf("ZOOMRIYA_ID_URL: %v", err)
	}
	auth := NewAuth(idBase + "/auth/jwks.json")
	auth.Client = idClient
	listener, err := localhttp.Listen(cfg.Socket)
	if err != nil {
		log.Fatalf("no socket to answer on: %v", err)
	}

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
		Handler:           NewAPI(engine, auth),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("zoomriya-model on %s", listener.Addr())
	err = server.Serve(listener)
	engine.Close()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
