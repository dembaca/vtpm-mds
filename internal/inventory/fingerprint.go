package inventory

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"os"
)

// Fingerprint is a digest of everything an inventory backend reads. Two loads
// from sources with the same fingerprint produce the same inventory, so the
// service reloads only when it changes (reload-when-guest-configs-change).
type Fingerprint [sha256.Size]byte

// FingerprintBuilder accumulates the inputs of a Fingerprint. Every field is
// length-prefixed, so no two different sequences of inputs share a digest.
type FingerprintBuilder struct {
	h hash.Hash
}

// NewFingerprint returns an empty builder.
func NewFingerprint() *FingerprintBuilder {
	return &FingerprintBuilder{h: sha256.New()}
}

func (b *FingerprintBuilder) field(kind byte, data []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(data)))
	b.h.Write([]byte{kind})
	b.h.Write(n[:])
	b.h.Write(data)
}

// AddString adds a value that is not a file, such as a symlink target.
func (b *FingerprintBuilder) AddString(s string) {
	b.field('s', []byte(s))
}

// AddFile adds a file's path and content. A file that does not exist, or that
// cannot be read, adds a marker instead, so its appearing, disappearing or
// becoming readable changes the digest; whether that state is an error is for
// the loader to decide when it reloads, not for the fingerprint.
func (b *FingerprintBuilder) AddFile(path string) {
	b.field('p', []byte(path))
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		b.field('c', data)
	case os.IsNotExist(err):
		b.field('m', nil)
	default:
		b.field('e', []byte(err.Error()))
	}
}

// Sum returns the digest of everything added so far.
func (b *FingerprintBuilder) Sum() Fingerprint {
	var f Fingerprint
	copy(f[:], b.h.Sum(nil))
	return f
}

// YAMLFingerprint is the fingerprint of the YAML inventory at path: the one
// file LoadYAML reads.
func YAMLFingerprint(path string) Fingerprint {
	b := NewFingerprint()
	b.AddFile(path)
	return b.Sum()
}
