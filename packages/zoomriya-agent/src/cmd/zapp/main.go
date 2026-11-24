// zapp makes ZOOMRIYA app packages (design §6.9), for zoomriya-apps' release
// builds. It isn't installed on the box.
//
//	zapp keygen NAME
//	    NAME.key: the private key (keep it secret, offline); prints the line
//	    for zoomriya-base's app-keys.
//	zapp pack -key NAME.key -manifest app.json -image image.tar -o photos-1.4.2.zapp units/*.service
//	    app.json: id, name, version, publisher, description, min_os, image
//	    ("image.tar"), start, migrate, data, databases. Files and units are
//	    filled in from the files given.
//	zapp verify -keys app-keys PACKAGE
//	    checks a package as the box would, and prints its manifest.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zoomriya.com/os/zoomriya-agent/zapp"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2:])
	case "pack":
		err = pack(os.Args[2:])
	case "verify":
		err = verify(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "zapp:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: zapp keygen NAME | pack -key K -manifest M -image I [-o OUT] UNITS... | verify -keys F PACKAGE")
	os.Exit(2)
}

func keygen(args []string) error {
	if len(args) != 1 {
		usage()
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(args[0]+".key", []byte(base64.StdEncoding.EncodeToString(priv)+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("ed25519 %s %s\n", base64.StdEncoding.EncodeToString(pub), filepath.Base(args[0]))
	return nil
}

func pack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	keyFile := fs.String("key", "", "the private key")
	manifestFile := fs.String("manifest", "", "the app's manifest")
	image := fs.String("image", "", "the image tar")
	out := fs.String("o", "", "the package to write")
	_ = fs.Parse(args)
	if *keyFile == "" || *manifestFile == "" || *image == "" {
		usage()
	}
	keyText, err := os.ReadFile(*keyFile)
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyText)))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return fmt.Errorf("%s isn't a zapp key", *keyFile)
	}
	data, err := os.ReadFile(*manifestFile)
	if err != nil {
		return err
	}
	var m zapp.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if *out == "" {
		*out = m.ID + "-" + m.Version + ".zapp"
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	if err := zapp.Pack(f, m, ed25519.PrivateKey(raw), *image, fs.Args()); err != nil {
		f.Close()
		os.Remove(*out)
		return err
	}
	fmt.Println(*out)
	return f.Close()
}

func verify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	keysFile := fs.String("keys", "", "trusted keys, as in zoomriya-base's app-keys")
	_ = fs.Parse(args)
	if *keysFile == "" || fs.NArg() != 1 {
		usage()
	}
	keys, err := os.ReadFile(*keysFile)
	if err != nil {
		return err
	}
	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()
	dir, err := os.MkdirTemp("", "zapp-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	m, err := zapp.Extract(f, zapp.ParseKeys(string(keys)), dir)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(m, "", "  ")
	fmt.Println(string(out))
	return nil
}
