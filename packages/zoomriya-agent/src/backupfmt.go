// The backup drive's encrypted files (design §6.10): each app's database and
// data, sealed with the box's backup key, the one the admin printed when they
// chose the drive. Photos and other files an app lists are copied as they are
// (backup.go); only these files need the key.
//
// The key: 15 random bytes (120 bits), printed as 24 letters and digits in six
// groups of four ("7KQ2-M9XA-…"), from Crockford's base32, so O and 0, I, L
// and 1 read as the same when typed back.
//
// A file: "ZOOMRIYA-BACKUP-1\n", a 32-byte salt, then chunks of at most 1 MiB,
// each a 4-byte big-endian length and AES-256-GCM's output. The file's own key
// is HKDF-SHA256 of the backup key with that salt, so the chunks' nonces are
// just their number; the last chunk is sealed with associated data 1 and the
// others with 0, so a file cut short or with chunks moved fails to open.
// Large files are split in parts of 1 GiB (<name>.001, .002, …), as a drive
// formatted FAT32 takes no file over 4 GiB.
package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	backupMagic      = "ZOOMRIYA-BACKUP-1\n"
	backupSaltBytes  = 32
	backupChunk      = 1 << 20
	backupKeyBytes   = 15
	backupKeyGroups  = 6
	backupPartBytes  = 1 << 30
	crockfordLetters = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

// errWrongKey: the key typed isn't this backup's, or the file was changed.
var errWrongKey = errors.New("that isn't this backup's key, or the backup was changed")

// newBackupKey: a new key, as printed.
func newBackupKey() string {
	raw := make([]byte, backupKeyBytes)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return formatBackupKey(raw)
}

func formatBackupKey(raw []byte) string {
	var bits uint64
	var have uint
	letters := []byte{}
	for _, b := range raw {
		bits = bits<<8 | uint64(b)
		have += 8
		for have >= 5 {
			letters = append(letters, crockfordLetters[(bits>>(have-5))&31])
			have -= 5
		}
	}
	groups := []string{}
	for i := 0; i < len(letters); i += len(letters) / backupKeyGroups {
		groups = append(groups, string(letters[i:i+len(letters)/backupKeyGroups]))
	}
	return strings.Join(groups, "-")
}

// parseBackupKey reads a key as an admin typed it: any case, with or without
// dashes and spaces, O for 0 and I or L for 1.
func parseBackupKey(typed string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '-', ' ', '\t':
			return -1
		case 'o', 'O':
			return '0'
		case 'i', 'I', 'l', 'L':
			return '1'
		}
		if r >= 'a' && r <= 'z' {
			return r - 'a' + 'A'
		}
		return r
	}, typed)
	if len(clean) != backupKeyBytes*8/5 {
		return nil, errors.New("a backup key is 24 letters and digits")
	}
	var bits uint64
	var have uint
	raw := []byte{}
	for _, r := range clean {
		value := strings.IndexRune(crockfordLetters, r)
		if value < 0 {
			return nil, fmt.Errorf("%q isn't in a backup key", r)
		}
		bits = bits<<5 | uint64(value)
		have += 5
		if have >= 8 {
			raw = append(raw, byte(bits>>(have-8)))
			have -= 8
		}
	}
	return raw, nil
}

func backupAEAD(key, salt []byte) (cipher.AEAD, error) {
	fileKey, err := hkdf.Key(sha256.New, key, salt, "zoomriya backup file", 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(fileKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func chunkNonce(n uint64) []byte {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], n)
	return nonce
}

// sealWriter seals what's written to it into out; Close seals the last chunk.
type sealWriter struct {
	out   io.Writer
	aead  cipher.AEAD
	buf   []byte
	count uint64
}

func newSealWriter(out io.Writer, key []byte) (*sealWriter, error) {
	salt := make([]byte, backupSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	aead, err := backupAEAD(key, salt)
	if err != nil {
		return nil, err
	}
	if _, err := out.Write(append([]byte(backupMagic), salt...)); err != nil {
		return nil, err
	}
	return &sealWriter{out: out, aead: aead, buf: make([]byte, 0, backupChunk)}, nil
}

func (w *sealWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		// A full chunk is sealed only once more follows, so Close knows the last.
		if len(w.buf) == backupChunk {
			if err := w.flush(false); err != nil {
				return written, err
			}
		}
		n := copy(w.buf[len(w.buf):backupChunk], p)
		w.buf = w.buf[:len(w.buf)+n]
		p = p[n:]
		written += n
	}
	return written, nil
}

func (w *sealWriter) flush(last bool) error {
	flag := []byte{0}
	if last {
		flag[0] = 1
	}
	sealed := w.aead.Seal(nil, chunkNonce(w.count), w.buf, flag)
	w.count++
	w.buf = w.buf[:0]
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(sealed)))
	if _, err := w.out.Write(size[:]); err != nil {
		return err
	}
	_, err := w.out.Write(sealed)
	return err
}

func (w *sealWriter) Close() error { return w.flush(true) }

// openReader opens what a sealWriter wrote; a wrong key, a change or a file
// cut short is errWrongKey.
type openReader struct {
	in    *bufio.Reader
	aead  cipher.AEAD
	plain []byte
	count uint64
	done  bool
}

func newOpenReader(in io.Reader, key []byte) (*openReader, error) {
	r := bufio.NewReaderSize(in, 1<<16)
	head := make([]byte, len(backupMagic)+backupSaltBytes)
	if _, err := io.ReadFull(r, head); err != nil || string(head[:len(backupMagic)]) != backupMagic {
		return nil, errors.New("this isn't a ZOOMRIYA backup file")
	}
	aead, err := backupAEAD(key, head[len(backupMagic):])
	if err != nil {
		return nil, err
	}
	return &openReader{in: r, aead: aead}, nil
}

func (r *openReader) Read(p []byte) (int, error) {
	for len(r.plain) == 0 {
		if r.done {
			return 0, io.EOF
		}
		if err := r.next(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.plain)
	r.plain = r.plain[n:]
	return n, nil
}

func (r *openReader) next() error {
	var size [4]byte
	if _, err := io.ReadFull(r.in, size[:]); err != nil {
		return errWrongKey // cut short: the last chunk never came
	}
	length := binary.BigEndian.Uint32(size[:])
	if length > backupChunk+uint32(r.aead.Overhead()) {
		return errWrongKey
	}
	sealed := make([]byte, length)
	if _, err := io.ReadFull(r.in, sealed); err != nil {
		return errWrongKey
	}
	_, peekErr := r.in.Peek(1)
	last := peekErr == io.EOF
	flag := []byte{0}
	if last {
		flag[0] = 1
	}
	plain, err := r.aead.Open(sealed[:0], chunkNonce(r.count), sealed, flag)
	if err != nil {
		return errWrongKey
	}
	r.count++
	r.plain, r.done = plain, last
	return nil
}

// sealBytes and openBytes: a small file sealed whole (the key check, the list
// of accounts).
func sealBytes(key, plain []byte) ([]byte, error) {
	var out bytes.Buffer
	w, err := newSealWriter(&out, key)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plain); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func openBytes(key, sealed []byte) ([]byte, error) {
	r, err := newOpenReader(bytes.NewReader(sealed), key)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

// --- parts ------------------------------------------------------------------------

// partWriter writes base.001, base.002, … of at most size bytes each, each
// synced to the drive before the next starts.
type partWriter struct {
	base  string
	size  int64
	n     int
	file  *os.File
	wrote int64
	Parts []string
}

func newPartWriter(base string, size int64) *partWriter {
	return &partWriter{base: base, size: size}
}

func (w *partWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		if w.file == nil || w.wrote == w.size {
			if err := w.closePart(); err != nil {
				return written, err
			}
			w.n++
			name := fmt.Sprintf("%s.%03d", w.base, w.n)
			f, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				return written, err
			}
			w.file, w.wrote = f, 0
			w.Parts = append(w.Parts, name)
		}
		room := w.size - w.wrote
		chunk := p
		if int64(len(chunk)) > room {
			chunk = chunk[:room]
		}
		n, err := w.file.Write(chunk)
		w.wrote += int64(n)
		written += n
		p = p[n:]
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

func (w *partWriter) closePart() error {
	if w.file == nil {
		return nil
	}
	err := w.file.Sync()
	if closeErr := w.file.Close(); err == nil {
		err = closeErr
	}
	w.file = nil
	return err
}

func (w *partWriter) Close() error { return w.closePart() }

// openParts reads base.001, base.002, … one after the other.
func openParts(base string) (io.ReadCloser, error) {
	files := []*os.File{}
	readers := []io.Reader{}
	for n := 1; ; n++ {
		f, err := os.Open(fmt.Sprintf("%s.%03d", base, n))
		if errors.Is(err, os.ErrNotExist) && n > 1 {
			break
		}
		if err != nil {
			for _, f := range files {
				f.Close()
			}
			return nil, err
		}
		files = append(files, f)
		readers = append(readers, f)
	}
	return &multiCloser{Reader: io.MultiReader(readers...), files: files}, nil
}

type multiCloser struct {
	io.Reader
	files []*os.File
}

func (m *multiCloser) Close() error {
	for _, f := range m.files {
		f.Close()
	}
	return nil
}
