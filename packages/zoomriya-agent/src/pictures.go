// Pictures on a USB stick, for an admin's desktop background (Box Desktop's
// Control Panel › Appearance). The agent reads them, since only it mounts
// sticks, and decodes them itself with Go's own JPEG and PNG decoders: Box
// Desktop gets back a fresh JPEG the agent made, scaled to a screen's size and
// turned the right way up, never the file from the stick.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // PNG, besides JPEG
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// Bigger files aren't read: a phone's photo is a few MB.
	pictureMaxFile = 40 << 20
	// Nor more pixels than this, whatever the file's size (100 MP).
	pictureMaxPixels = 100_000_000
	// A background is at most this wide or tall (4K); bigger is scaled down.
	pictureMaxSide = 3840
	// The previews in the list.
	pictureThumbSide = 360
	// How many pictures a look on the sticks lists.
	pictureMaxFound = 48
)

// Folders on a stick that are never pictures of the family's.
var pictureSkipFolders = map[string]bool{
	"system volume information": true, "$recycle.bin": true, "lost+found": true,
}

type FoundPicture struct {
	Stick  string `json:"stick"`
	Device string `json:"device"`
	// Where it is on the stick: "beach.jpg" or "Photos/beach.jpg".
	File   string `json:"file"`
	Size   int64  `json:"size_bytes"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// A small JPEG to show it by, base64.
	Thumb string `json:"thumb"`
}

// picturesRequest: Action "usb" lists the pictures on the sticks plugged in
// (their top folder and the folders in it), each with a preview; "read" makes
// one of them (Device, Name) into a background.
func (a *Agent) picturesRequest(req Request) (Response, error) {
	switch deref(req.Action) {
	case "usb":
		found, err := a.usbPictures()
		if err != nil {
			return nil, err
		}
		return Response{"found": found}, nil
	case "read":
		if deref(req.Device) == "" || deref(req.Name) == "" {
			return nil, refuse("Which picture?")
		}
		var resp Response
		err := a.withStick(deref(req.Device), false, func(dir string) error {
			path, err := picturePath(dir, deref(req.Name))
			if err != nil {
				return err
			}
			img, err := loadPicture(path, pictureMaxSide)
			if err != nil {
				return err
			}
			var out bytes.Buffer
			if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 90}); err != nil {
				return err
			}
			b := img.Bounds()
			resp = Response{"jpeg": base64.StdEncoding.EncodeToString(out.Bytes()), "width": b.Dx(), "height": b.Dy()}
			return nil
		})
		return resp, err
	}
	return nil, refuse("Not something pictures can do.")
}

// isPicture: a name a JPEG or PNG would have.
func isPicture(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png":
		return !strings.HasPrefix(name, ".")
	}
	return false
}

// usbPictures looks in each stick's top folder and the folders in it, for
// JPEG and PNG files: regular files only, nothing followed or run.
func (a *Agent) usbPictures() ([]FoundPicture, error) {
	sticks, err := a.usbSticks()
	if err != nil {
		return nil, err
	}
	found := []FoundPicture{}
	for _, stick := range sticks {
		if len(found) >= pictureMaxFound {
			break
		}
		_ = a.withStick(stick.Device, false, func(dir string) error {
			for _, name := range picturesIn(dir) {
				if len(found) >= pictureMaxFound {
					break
				}
				path := filepath.Join(dir, name)
				info, err := os.Lstat(path)
				if err != nil {
					continue
				}
				img, err := loadPicture(path, pictureThumbSide)
				if err != nil {
					continue // not a picture after all, or too big
				}
				var thumb bytes.Buffer
				if jpeg.Encode(&thumb, img, &jpeg.Options{Quality: 80}) != nil {
					continue
				}
				w, h := pictureSize(path)
				found = append(found, FoundPicture{
					Stick: stick.Label, Device: stick.Device, File: name, Size: info.Size(),
					Width: w, Height: h, Thumb: base64.StdEncoding.EncodeToString(thumb.Bytes()),
				})
			}
			return nil
		})
	}
	return found, nil
}

// picturesIn: the pictures' names in dir and one folder down, by name.
func picturesIn(dir string) []string {
	names := []string{}
	add := func(rel string) {
		info, err := os.Lstat(filepath.Join(dir, rel))
		if err == nil && info.Mode().IsRegular() && info.Size() <= pictureMaxFile && isPicture(filepath.Base(rel)) {
			names = append(names, rel)
		}
	}
	top, _ := os.ReadDir(dir)
	sort.Slice(top, func(i, j int) bool { return strings.ToLower(top[i].Name()) < strings.ToLower(top[j].Name()) })
	var folders []string
	for _, e := range top {
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		if e.IsDir() {
			if !strings.HasPrefix(e.Name(), ".") && !pictureSkipFolders[strings.ToLower(e.Name())] {
				folders = append(folders, e.Name())
			}
			continue
		}
		add(e.Name())
	}
	for _, folder := range folders {
		inside, _ := os.ReadDir(filepath.Join(dir, folder))
		sort.Slice(inside, func(i, j int) bool { return strings.ToLower(inside[i].Name()) < strings.ToLower(inside[j].Name()) })
		for _, e := range inside {
			if e.Type().IsRegular() {
				add(folder + "/" + e.Name())
			}
		}
	}
	return names
}

// picturePath: name on the stick mounted at dir, if it's one usbPictures would
// list: in the top folder or one down, no links on the way, a regular file.
func picturePath(dir, name string) (string, error) {
	parts := strings.Split(name, "/")
	if !filepath.IsLocal(name) || len(parts) > 2 || !isPicture(parts[len(parts)-1]) {
		return "", refuse("That isn't a picture on the USB stick.")
	}
	path := dir
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return "", refuse("That picture isn't on the USB stick any more.")
		}
		last := i == len(parts)-1
		if (last && !info.Mode().IsRegular()) || (!last && !info.IsDir()) {
			return "", refuse("That isn't a picture on the USB stick.")
		}
		if last && info.Size() > pictureMaxFile {
			return "", refuse("That picture is too big.")
		}
	}
	return path, nil
}

// pictureSize: the picture's width and height as it's seen (turned by its
// EXIF orientation), or 0, 0.
func pictureSize(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	if orientation(path) >= 5 {
		return cfg.Height, cfg.Width
	}
	return cfg.Width, cfg.Height
}

// loadPicture decodes a JPEG or PNG, refusing anything bigger than
// pictureMaxPixels before decoding it, shrinks it to fit side × side, and
// turns it the right way up.
func loadPicture(path string, side int) (*image.RGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > pictureMaxFile {
		return nil, refuse("The box couldn't read that picture.")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, refuse("That isn't a JPEG or PNG picture.")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > pictureMaxPixels {
		return nil, refuse("That picture is too big.")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, refuse("The box couldn't read that picture.")
	}
	w, h := fit(cfg.Width, cfg.Height, side)
	small := shrink(img, w, h)
	if format == "jpeg" {
		return turn(small, exifOrientation(data)), nil
	}
	return small, nil
}

// fit: w × h scaled down, keeping its shape, to fit side × side.
func fit(w, h, side int) (int, int) {
	if w <= side && h <= side {
		return w, h
	}
	if w >= h {
		return side, max(1, h*side/w)
	}
	return max(1, w*side/h), side
}

// shrink averages each w × h pixel's share of src (never larger than src).
// Each pixel of src is read once; a JPEG's and a PNG's usual kinds directly.
func shrink(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sums := make([]uint64, w*4)
	at := pixelReader(src)
	for ty := 0; ty < h; ty++ {
		clear(sums)
		y0, y1 := ty*sh/h, max((ty+1)*sh/h, ty*sh/h+1)
		for sy := y0; sy < y1; sy++ {
			for sx := 0; sx < sw; sx++ {
				r, g, bl := at(b.Min.X+sx, b.Min.Y+sy)
				i := (sx * w / sw) * 4
				sums[i] += uint64(r)
				sums[i+1] += uint64(g)
				sums[i+2] += uint64(bl)
				sums[i+3]++
			}
		}
		for tx := 0; tx < w; tx++ {
			n := max(sums[tx*4+3], 1)
			dst.SetRGBA(tx, ty, color.RGBA{uint8(sums[tx*4] / n), uint8(sums[tx*4+1] / n), uint8(sums[tx*4+2] / n), 255})
		}
	}
	return dst
}

// pixelReader: a picture's pixel as 8-bit red, green and blue, over black
// where it's see-through.
func pixelReader(src image.Image) func(x, y int) (uint8, uint8, uint8) {
	switch p := src.(type) {
	case *image.YCbCr:
		return func(x, y int) (uint8, uint8, uint8) {
			return color.YCbCrToRGB(p.Y[p.YOffset(x, y)], p.Cb[p.COffset(x, y)], p.Cr[p.COffset(x, y)])
		}
	case *image.NRGBA:
		return func(x, y int) (uint8, uint8, uint8) {
			i := p.PixOffset(x, y)
			a := uint16(p.Pix[i+3])
			return uint8(uint16(p.Pix[i]) * a / 255), uint8(uint16(p.Pix[i+1]) * a / 255), uint8(uint16(p.Pix[i+2]) * a / 255)
		}
	case *image.RGBA:
		return func(x, y int) (uint8, uint8, uint8) {
			i := p.PixOffset(x, y)
			return p.Pix[i], p.Pix[i+1], p.Pix[i+2]
		}
	}
	return func(x, y int) (uint8, uint8, uint8) {
		r, g, b, _ := src.At(x, y).RGBA()
		return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
	}
}

// turn: src as its EXIF orientation (1–8) says it should be seen.
func turn(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.SetRGBA(dx, dy, src.RGBAAt(x, y))
		}
	}
	return dst
}

// orientation: the EXIF orientation of the JPEG at path, or 1.
func orientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()
	head := make([]byte, 128<<10)
	n, _ := f.Read(head)
	return exifOrientation(head[:n])
}

// exifOrientation reads the Orientation tag (0x0112) from a JPEG's EXIF
// segment, or 1 (as it is) when there's none or it can't be read.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) && data[i] == 0xFF {
		marker := data[i+1]
		length := int(binary.BigEndian.Uint16(data[i+2:]))
		if marker == 0xDA || length < 2 || i+2+length > len(data) {
			return 1 // the picture itself starts: no EXIF before it
		}
		segment := data[i+4 : i+2+length]
		if marker == 0xE1 && len(segment) > 14 && string(segment[:6]) == "Exif\x00\x00" {
			return tiffOrientation(segment[6:])
		}
		i += 2 + length
	}
	return 1
}

func tiffOrientation(t []byte) int {
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	count := int(order.Uint16(t[ifd:]))
	for e := 0; e < count; e++ {
		at := ifd + 2 + e*12
		if at+12 > len(t) {
			return 1
		}
		if order.Uint16(t[at:]) == 0x0112 {
			if o := int(order.Uint16(t[at+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}
