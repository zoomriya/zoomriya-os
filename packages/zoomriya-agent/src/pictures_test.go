package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A w × h picture, red on the left half and blue on the right.
func testPicture(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{220, 30, 30, 255}
			if x >= w/2 {
				c = color.RGBA{30, 30, 220, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func jpegOf(t *testing.T, img image.Image) []byte {
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, nil); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func pngOf(t *testing.T, img image.Image) []byte {
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// withOrientation puts an EXIF segment saying o right after a JPEG's start.
func withOrientation(data []byte, o byte) []byte {
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, o, 0, 0, 0, 0, 0, 0}
	body := append([]byte("Exif\x00\x00"), tiff...)
	n := len(body) + 2
	segment := append([]byte{0xFF, 0xE1, byte(n >> 8), byte(n)}, body...)
	return append(append([]byte{0xFF, 0xD8}, segment...), data[2:]...)
}

func pictureBox(t *testing.T) (*Agent, *fakeBox) {
	agent, box, _ := accessBox(t)
	box.answers = map[string]string{"sticks": sticksJSON}
	return agent, box
}

func (b *fakeBox) writeBytes(rel string, data []byte) {
	path := filepath.Join(b.root, rel)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func TestUSBPicturesOnlyPictures(t *testing.T) {
	agent, box := pictureBox(t)
	stick := usbMount + "/"
	box.writeBytes(stick+"beach.jpg", jpegOf(t, testPicture(800, 600)))
	box.writeBytes(stick+"Photos/garden.PNG", pngOf(t, testPicture(64, 64)))
	// Not pictures, whatever they're called.
	box.write(stick+"script.jpg", "#!/bin/sh\nrm -rf /\n")
	box.write(stick+"page.png", "<html><script>alert(1)</script></html>")
	box.write(stick+"drawing.svg", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	box.write(stick+"run.sh", "#!/bin/sh\n")
	box.write(stick+"notes.txt", "hello\n")
	box.writeBytes(stick+"anim.gif", []byte("GIF89a"))
	box.writeBytes(stick+".hidden.jpg", jpegOf(t, testPicture(10, 10)))
	box.writeBytes(stick+"Photos/deeper/far.jpg", jpegOf(t, testPicture(10, 10)))
	box.writeBytes(stick+"System Volume Information/x.jpg", jpegOf(t, testPicture(10, 10)))
	if err := os.Symlink("/etc/shadow", filepath.Join(box.root, stick, "link.jpg")); err != nil {
		t.Fatal(err)
	}

	resp, err := agent.Handle(Request{Op: "pictures", Action: str("usb")})
	if err != nil {
		t.Fatal(err)
	}
	found := resp["found"].([]FoundPicture)
	var names []string
	for _, p := range found {
		names = append(names, p.File)
	}
	if strings.Join(names, " ") != "beach.jpg Photos/garden.PNG" {
		t.Fatalf("found %v", names)
	}
	if found[0].Width != 800 || found[0].Height != 600 || found[0].Stick != "ANNA" || found[0].Device != "/dev/sdb1" {
		t.Errorf("beach = %+v", found[0])
	}
	thumb, _ := base64.StdEncoding.DecodeString(found[0].Thumb)
	cfg, format, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil || format != "jpeg" || cfg.Width != pictureThumbSide || cfg.Height != 270 {
		t.Errorf("thumb %v %s %v", cfg, format, err)
	}
	if !ranCommand(box, "mount -t vfat -o nosuid,nodev,noexec,noatime,ro /dev/sdb1 "+filepath.Join(box.root, usbMount)) {
		t.Errorf("ran %v", box.ran)
	}
}

func TestReadPictureIsANewJPEG(t *testing.T) {
	agent, box := pictureBox(t)
	// A real picture with something else stuck on its end: only the picture comes back.
	data := append(jpegOf(t, testPicture(5000, 2000)), []byte("#!/bin/sh\necho hidden\n")...)
	box.writeBytes(usbMount+"/Photos/wide.jpg", data)
	resp, err := agent.Handle(Request{Op: "pictures", Action: str("read"), Device: str("/dev/sdb1"), Name: str("Photos/wide.jpg")})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := base64.StdEncoding.DecodeString(resp["jpeg"].(string))
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil || format != "jpeg" {
		t.Fatalf("%s %v", format, err)
	}
	if b := img.Bounds(); b.Dx() != pictureMaxSide || b.Dy() != 1536 {
		t.Errorf("size %v", b)
	}
	if bytes.Contains(out, []byte("hidden")) {
		t.Error("what was stuck on the file came back")
	}
	r, _, b, _ := img.At(10, 10).RGBA()
	if r>>8 < 150 || b>>8 > 100 {
		t.Errorf("left isn't red: %v", img.At(10, 10))
	}
}

func TestReadPictureRefuses(t *testing.T) {
	agent, box := pictureBox(t)
	box.write(usbMount+"/script.jpg", "#!/bin/sh\nrm -rf /\n")
	box.write(usbMount+"/drawing.svg", "<svg/>")
	box.writeBytes(usbMount+"/a/b/deep.jpg", jpegOf(t, testPicture(8, 8)))
	box.writeBytes(usbMount+"/ok.jpg", jpegOf(t, testPicture(8, 8)))
	if err := os.Symlink("/etc", filepath.Join(box.root, usbMount, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(box.root, usbMount, "ok.jpg"), filepath.Join(box.root, usbMount, "link.jpg")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"script.jpg", "drawing.svg", "a/b/deep.jpg", "../ok.jpg", "/etc/passwd.jpg",
		"linked/x.jpg", "link.jpg", "missing.jpg", ""} {
		_, err := agent.Handle(Request{Op: "pictures", Action: str("read"), Device: str("/dev/sdb1"), Name: str(name)})
		if err == nil {
			t.Errorf("%q was read", name)
		}
	}
	if _, err := agent.Handle(Request{Op: "pictures", Action: str("read"), Device: str("/dev/sdb1"), Name: str("ok.jpg")}); err != nil {
		t.Errorf("ok.jpg: %v", err)
	}
	if _, err := agent.Handle(Request{Op: "pictures", Action: str("delete")}); err == nil {
		t.Error("an unknown action was done")
	}
}

func TestTooManyPixelsIsRefusedBeforeDecoding(t *testing.T) {
	dir := t.TempDir()
	// A PNG's header saying 20000 × 20000: refused from the header alone.
	var out bytes.Buffer
	_ = png.Encode(&out, image.NewGray(image.Rect(0, 0, 1, 1)))
	data := out.Bytes()
	data[16], data[17], data[18], data[19] = 0, 0, 0x4E, 0x20
	data[20], data[21], data[22], data[23] = 0, 0, 0x4E, 0x20
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	path := filepath.Join(dir, "huge.png")
	_ = os.WriteFile(path, data, 0o644)
	if _, err := loadPicture(path, pictureMaxSide); err == nil || !strings.Contains(err.Error(), "too big") {
		t.Errorf("err = %v", err)
	}
}

func TestPhotosTurnedTheRightWayUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "phone.jpg")
	// Taken holding the phone upright: stored 80 × 40, to be turned 90°.
	_ = os.WriteFile(path, withOrientation(jpegOf(t, testPicture(80, 40)), 6), 0o644)
	img, err := loadPicture(path, pictureMaxSide)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 80 {
		t.Errorf("size %v", b)
	}
	// Red was on the left, so it's on top once turned clockwise.
	if c := img.RGBAAt(20, 5); c.R < 150 || c.B > 100 {
		t.Errorf("top isn't red: %v", c)
	}
	if w, h := pictureSize(path); w != 40 || h != 80 {
		t.Errorf("listed as %d × %d", w, h)
	}
	if exifOrientation([]byte("not a jpeg")) != 1 || exifOrientation([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0, 2}) != 1 {
		t.Error("nonsense isn't as it is")
	}
}
