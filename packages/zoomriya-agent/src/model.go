// model (box-desktop.md §4, Myira; BD-17): Myira's model on a box that has
// none, fetched when an admin asks for it the first time the panel opens,
// not with the OS. Downloaded, or copied from a USB stick; either way each
// file is checked against the size and SHA-256 written here before the box's
// model service (zoomriya-model) sees it.
//
// The download runs as its own account, zoomriya-model-fetch, the one the
// model's declared connection names (zoomriya-model's
// /etc/zoomriya/outbound.d/model.json): the firewall lets it out only while
// that connection is allowed (design §8.2). A copy from a stick needs no
// internet. Both run as their own unit, so a closed panel or a restarted
// agent doesn't stop them, and both write <file>.part beside the models,
// which is how far they've got.
//
//	model                      what's there, what's running, how far, and why not
//	model download  (Name)     fetch the model (e2b, the default, or e4b)
//	model usb                  the model's files on the USB sticks
//	model copy      (Device, Name) copy them from that stick
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	modelDir        = "data/models/llms"
	modelFetchUser  = "zoomriya-model-fetch"
	modelFetchUnit  = "zoomriya-model-fetch"
	modelCopyUnit   = "zoomriya-model-copy"
	modelDeclaredID = "model"
	modelSource     = "https://huggingface.co/lmstudio-community/gemma-4-%s-it-GGUF/resolve/main/%s"
)

type modelFile struct {
	Name   string
	Size   int64
	SHA256 string
}

// modelFiles: each model the box offers, its chat file and its vision file,
// as lmstudio-community publishes them (Gemma 4, Apache-2.0).
var modelFiles = map[string][]modelFile{
	"e2b": {
		{"gemma-4-E2B-it-Q4_K_M.gguf", 3427880384, "71e6e8cb64a76da1a734fb8f6ba389d2784950d9902f5e87244fca0a15190c94"},
		{"mmproj-gemma-4-E2B-it-BF16.gguf", 986833248, "04125f8e2a1135824e48064c32157356704e6681a9f3af669af321a3c70c39d7"},
	},
	"e4b": {
		{"gemma-4-E4B-it-Q4_K_M.gguf", 5335291936, "0ffb122c8b6921f13cbc34186e052524d0b5803b17f4867b7197a561400b3770"},
		{"mmproj-gemma-4-E4B-it-BF16.gguf", 991551840, "bdfc4935857658dfdc8d1adebfa3897c2fec8b3323410f4f73ade36d6efa02be"},
	},
}

func modelVariant(name *string) (string, error) {
	v := strings.ToLower(deref(name))
	if v == "" {
		v = "e2b"
	}
	if _, ok := modelFiles[v]; !ok {
		return "", refuse("The box doesn't know a model called %q.", v)
	}
	return v, nil
}

func (a *Agent) modelRequest(req Request) (Response, error) {
	switch deref(req.Action) {
	case "":
		return a.modelState(), nil
	case "download":
		v, err := modelVariant(req.Name)
		if err != nil {
			return nil, err
		}
		if busy := a.modelBusy(); busy != "" {
			return nil, refuse("The model is already being fetched.")
		}
		if why := a.modelBlocked(); why != "" {
			return nil, refuse("%s", why)
		}
		if _, err := a.Run("systemd-run", "--quiet", "--collect", "--unit="+modelFetchUnit,
			"--description=Download Myira's model", "--property=Type=oneshot",
			"--property=User="+modelFetchUser, "--property=Group="+modelFetchUser,
			"--property=NoNewPrivileges=yes", "--property=ProtectSystem=strict",
			"--property=ReadWritePaths=/"+modelDir, "--property=ProtectHome=yes", "--property=PrivateTmp=yes",
			"--", agentPath, "model-fetch", v); err != nil {
			return nil, err
		}
		return a.modelState(), nil
	case "usb":
		found, err := a.modelOnSticks()
		if err != nil {
			return nil, err
		}
		return Response{"found": found}, nil
	case "copy":
		v, err := modelVariant(req.Name)
		if err != nil {
			return nil, err
		}
		if deref(req.Device) == "" {
			return nil, refuse("Which USB stick?")
		}
		if busy := a.modelBusy(); busy != "" {
			return nil, refuse("The model is already being fetched.")
		}
		if _, err := a.Run("systemd-run", "--quiet", "--collect", "--unit="+modelCopyUnit,
			"--description=Copy Myira's model from a USB stick", "--property=Type=oneshot",
			"--", agentPath, "model-copy", deref(req.Device), v, req.By); err != nil {
			return nil, err
		}
		return a.modelState(), nil
	}
	return nil, refuse("Not something the model can do.")
}

// modelBusy: "download" or "copy" while one runs, else "".
func (a *Agent) modelBusy() string {
	for unit, kind := range map[string]string{modelFetchUnit: "download", modelCopyUnit: "copy"} {
		out, _ := a.Run("systemctl", "is-active", unit+".service")
		if state := strings.TrimSpace(string(out)); state == "active" || state == "activating" {
			return kind
		}
	}
	return ""
}

// modelBlocked: why the download can't reach the internet now, or "".
func (a *Agent) modelBlocked() string {
	for _, d := range a.declared(a.internetChoice()) {
		if d.ID == modelDeclaredID {
			if d.Allowed {
				return ""
			}
			return "Internet access for Myira's model is off (Control Panel › Security › Internet Access): " + d.Blocked + "."
		}
	}
	return "The box's model service isn't installed."
}

// modelState: each model, whether it's all there, and how far a download or
// copy has got.
func (a *Agent) modelState() Response {
	models := []Response{}
	for _, v := range []string{"e2b", "e4b"} {
		var have, total int64
		complete := true
		for _, f := range modelFiles[v] {
			total += f.Size
			if info, err := os.Stat(a.path(filepath.Join(modelDir, f.Name))); err == nil && info.Size() == f.Size {
				have += f.Size
				continue
			}
			complete = false
			if info, err := os.Stat(a.path(filepath.Join(modelDir, f.Name+".part"))); err == nil {
				have += min(info.Size(), f.Size)
			}
		}
		models = append(models, Response{"name": v, "complete": complete, "bytes": have, "total": total})
	}
	state := Response{"models": models, "busy": a.modelBusy()}
	if why := a.modelBlocked(); why != "" {
		state["download_blocked"] = why
	}
	if failed := a.readTrimmed(filepath.Join(modelDir, ".failed")); failed != "" {
		state["failed"] = failed
	}
	return state
}

type FoundModel struct {
	Stick  string `json:"stick"`
	Device string `json:"device"`
	Name   string `json:"name"`
}

// modelOnSticks: the sticks holding all of a model's files, at the top or
// one folder down.
func (a *Agent) modelOnSticks() ([]FoundModel, error) {
	sticks, err := a.usbSticks()
	if err != nil {
		return nil, err
	}
	found := []FoundModel{}
	for _, stick := range sticks {
		_ = a.withStick(stick.Device, false, func(dir string) error {
			for _, v := range []string{"e2b", "e4b"} {
				all := true
				for _, f := range modelFiles[v] {
					all = all && modelFileOn(dir, f) != ""
				}
				if all {
					found = append(found, FoundModel{Stick: stick.Label, Device: stick.Device, Name: v})
				}
			}
			return nil
		})
	}
	return found, nil
}

// modelFileOn: the file's path on the stick when it's there at its size.
func modelFileOn(dir string, f modelFile) string {
	candidates, _ := filepath.Glob(filepath.Join(dir, "*", f.Name))
	for _, path := range append([]string{filepath.Join(dir, f.Name)}, candidates...) {
		if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Size() == f.Size {
			return path
		}
	}
	return ""
}

// ---- the jobs (each its own unit) ---------------------------------------------------

// keepModelFile writes from into <name>.part, from where it left off when it
// can, checks its size and SHA-256, then puts it in place.
func (a *Agent) keepModelFile(f modelFile, from func(offset int64) (io.ReadCloser, error)) error {
	final := a.path(filepath.Join(modelDir, f.Name))
	if info, err := os.Stat(final); err == nil && info.Size() == f.Size {
		return nil
	}
	part := final + ".part"
	hash := sha256.New()
	var offset int64
	if existing, err := os.Open(part); err == nil {
		offset, _ = io.Copy(hash, existing)
		existing.Close()
	}
	if offset > f.Size {
		os.Remove(part)
		offset = 0
		hash.Reset()
	}
	if offset < f.Size {
		body, err := from(offset)
		if err != nil {
			return err
		}
		defer body.Close()
		out, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		_, err = io.Copy(io.MultiWriter(out, hash), io.LimitReader(body, f.Size-offset))
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
	}
	if info, err := os.Stat(part); err != nil || info.Size() != f.Size {
		return fmt.Errorf("%s stopped before the end", f.Name)
	}
	if hex.EncodeToString(hash.Sum(nil)) != f.SHA256 {
		os.Remove(part)
		return fmt.Errorf("%s isn't the file it should be (its SHA-256 doesn't match)", f.Name)
	}
	return os.Rename(part, final)
}

func (a *Agent) modelJob(v string, keep func(f modelFile) error) error {
	os.Remove(a.path(filepath.Join(modelDir, ".failed")))
	for _, f := range modelFiles[v] {
		if err := keep(f); err != nil {
			os.WriteFile(a.path(filepath.Join(modelDir, ".failed")), []byte(err.Error()+"\n"), 0o644)
			return err
		}
	}
	return nil
}

// modelFetchJob: agent model-fetch VARIANT, as zoomriya-model-fetch.
func modelFetchJob(v string) int {
	if _, ok := modelFiles[v]; !ok {
		return 2
	}
	a := NewAgent()
	client := &http.Client{Timeout: 0, Transport: &http.Transport{ResponseHeaderTimeout: time.Minute}}
	variant := strings.ToUpper(v)
	err := a.modelJob(v, func(f modelFile) error {
		return a.keepModelFile(f, func(offset int64) (io.ReadCloser, error) {
			request, _ := http.NewRequest(http.MethodGet, fmt.Sprintf(modelSource, variant, f.Name), nil)
			request.Header.Set("User-Agent", "ZOOMRIYA OS")
			if offset > 0 {
				request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
			}
			response, err := client.Do(request)
			if err != nil {
				return nil, errors.New("the box couldn't reach the model's download")
			}
			if response.StatusCode == http.StatusOK && offset > 0 {
				// No resuming: start again.
				response.Body.Close()
				os.Remove(a.path(filepath.Join(modelDir, f.Name+".part")))
				return nil, errors.New("the download has to start again")
			}
			if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
				response.Body.Close()
				return nil, fmt.Errorf("the model's download answered %s", response.Status)
			}
			return response.Body, nil
		})
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// modelCopyJob: agent model-copy DEVICE VARIANT BY, as root, from a stick.
func modelCopyJob(device, v, by string) int {
	if _, ok := modelFiles[v]; !ok {
		return 2
	}
	a := NewAgent()
	err := a.withStick(device, false, func(dir string) error {
		return a.modelJob(v, func(f modelFile) error {
			path := modelFileOn(dir, f)
			if path == "" {
				return fmt.Errorf("%s isn't on the USB stick", f.Name)
			}
			return a.keepModelFile(f, func(offset int64) (io.ReadCloser, error) {
				in, err := os.Open(path)
				if err != nil {
					return nil, err
				}
				if _, err := in.Seek(offset, io.SeekStart); err != nil {
					in.Close()
					return nil, err
				}
				return in, nil
			})
		})
	})
	NewJournal().Record(AuditEntry{Op: "model-copy", By: by, Detail: v, Result: resultOf(err)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func resultOf(err error) string {
	if err != nil {
		return err.Error()
	}
	return "ok"
}
