package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func declareModel(box *fakeBox) {
	box.write("etc/zoomriya/outbound.d/model.json", `{"id":"model","account":"zoomriya-model-fetch","name":"Myira's model",`+
		`"feature":"Myira","what":"x","why":"y","when":"z","hosts":["huggingface.co"],"optional":true}`)
}

func TestTheModelIsFetchedAsItsOwnAccount(t *testing.T) {
	agent, box := newFakeBox(t)
	if _, err := agent.Handle(Request{Op: "model", Action: str("download"), By: "ada"}); err == nil ||
		!strings.Contains(err.Error(), "isn't installed") {
		t.Errorf("no declaration: %v", err)
	}
	declareModel(box)
	box.write("etc/zoomriya/internet.json", `{"apps_offline":true}`)
	if _, err := agent.Handle(Request{Op: "model", Action: str("download"), By: "ada"}); err == nil ||
		!strings.Contains(err.Error(), "Internet access for Myira's model is off") {
		t.Errorf("apps offline: %v", err)
	}
	box.write("etc/zoomriya/internet.json", `{}`)
	if _, err := agent.Handle(Request{Op: "model", Action: str("download"), Name: str("e4b"), By: "ada"}); err != nil {
		t.Fatal(err)
	}
	ran := ""
	for _, r := range box.ran {
		if r[0] == "systemd-run" {
			ran = strings.Join(r, " ")
		}
	}
	for _, want := range []string{"--unit=zoomriya-model-fetch", "--property=User=zoomriya-model-fetch",
		"--property=ReadWritePaths=/data/models/llms", agentPath + " model-fetch e4b"} {
		if !strings.Contains(ran, want) {
			t.Errorf("%q not in %q", want, ran)
		}
	}
	if _, err := agent.Handle(Request{Op: "model", Action: str("download"), Name: str("big"), By: "ada"}); err == nil {
		t.Error("an unknown model was fetched")
	}
}

func TestTheModelsStateCountsWhatIsThere(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("data/models/llms/mmproj-gemma-4-E2B-it-BF16.gguf.part", "0123456789")
	resp, err := agent.Handle(Request{Op: "model"})
	if err != nil {
		t.Fatal(err)
	}
	e2b := resp["models"].([]Response)[0]
	if e2b["name"] != "e2b" || e2b["complete"] != false || e2b["bytes"] != int64(10) || e2b["total"] != int64(3427880384+986833248) {
		t.Errorf("e2b: %v", e2b)
	}
}

func TestAModelFileIsKeptOnlyWhenItsHashMatches(t *testing.T) {
	agent, box := newFakeBox(t)
	content := "the model's bytes"
	sum := sha256.Sum256([]byte(content))
	f := modelFile{"tiny.gguf", int64(len(content)), hex.EncodeToString(sum[:])}
	// Half was fetched before; the rest comes from where it stopped.
	box.write("data/models/llms/tiny.gguf.part", content[:5])
	asked := int64(-1)
	err := agent.keepModelFile(f, func(offset int64) (io.ReadCloser, error) {
		asked = offset
		return io.NopCloser(strings.NewReader(content[offset:])), nil
	})
	if err != nil || asked != 5 {
		t.Fatalf("resume: %v from %d", err, asked)
	}
	if got, _ := os.ReadFile(filepath.Join(box.root, "data/models/llms/tiny.gguf")); string(got) != content {
		t.Errorf("kept %q", got)
	}
	bad := modelFile{"bad.gguf", int64(len(content)), strings.Repeat("0", 64)}
	err = agent.keepModelFile(bad, func(int64) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(content)), nil })
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Errorf("a wrong file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(box.root, "data/models/llms/bad.gguf")); err == nil {
		t.Error("a wrong file was kept")
	}
}
