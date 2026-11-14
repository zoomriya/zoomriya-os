// Starting llama.cpp's llama-server with one model, on loopback, behind a
// key made at each start and known only to this service (passed in its
// environment, which other accounts can't read, not on its command line).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

// LoadTimeout: how long a model may take to load (E4B on a slow disk).
const LoadTimeout = 10 * time.Minute

func startLlamaServer(cfg Config) Starter {
	return func(model, vision string) (*Running, error) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		host, port := splitHostPort(cfg.ServerAddr)
		// Half the cores: the box has other work, and llama.cpp gains little
		// from hyper-threads.
		threads := strconv.Itoa(max(1, runtime.NumCPU()/2))
		args := []string{
			"--model", model,
			"--host", host, "--port", port,
			"--ctx-size", strconv.Itoa(cfg.Context),
			"--threads", threads, "--threads-batch", threads,
			"--batch-size", "1024",
			// One answer at a time; the front decides whose.
			"--parallel", "1",
			// The model's own chat template (Gemma 4's), as the apps had.
			"--jinja",
			// An answer, never the model's thinking.
			"--reasoning", "off",
			// Nothing kept or shown: no web page, no slots (they hold the
			// prompts), no prompt cache beyond the one answer's own.
			"--no-webui", "--no-slots", "--cache-ram", "0",
			"--offline",
			"--log-verbosity", "1",
		}
		if vision != "" {
			args = append(args, "--mmproj", vision, "--no-mmproj-offload")
		} else {
			args = append(args, "--no-mmproj")
		}
		cmd := exec.Command(cfg.Server, args...)
		cmd.Env = append(os.Environ(), "LLAMA_API_KEY="+hex.EncodeToString(key))
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		// It goes when this service goes, however that happens.
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		done := make(chan struct{})
		var exit error
		go func() {
			exit = cmd.Wait()
			close(done)
		}()
		stop := func() {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		}
		running := &Running{
			Base: "http://" + cfg.ServerAddr,
			Key:  hex.EncodeToString(key),
			Sees: vision != "",
			Done: done,
			Stop: stop,
		}
		if err := waitHealthy(running.Base, done, LoadTimeout); err != nil {
			stop()
			if exit != nil {
				return nil, fmt.Errorf("%w (%v)", err, exit)
			}
			return nil, err
		}
		return running, nil
	}
}

// waitHealthy waits until llama-server says the model is loaded.
func waitHealthy(base string, done <-chan struct{}, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second}
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-done:
			return errors.New("llama-server stopped while loading the model")
		case <-ctx.Done():
			return errors.New("the model took too long to load")
		case <-tick.C:
			response, err := client.Get(base + "/health")
			if err == nil {
				response.Body.Close()
				if response.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

func splitHostPort(addr string) (string, string) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:]
		}
	}
	return addr, "8071"
}
