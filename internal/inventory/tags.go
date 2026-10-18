package inventory

import "regexp"

// tagKey and tagValue are the characters an instance tag may carry. They are a
// subset of what a Proxmox tag can hold, which also keeps a key a safe path
// segment of /latest/meta-data/tags/instance/<key>. A key has no dot because the
// first dot after it ends it in a Proxmox tag; a value may have dots.
var (
	tagKey   = regexp.MustCompile(`^[A-Za-z0-9_+-]{1,63}$`)
	tagValue = regexp.MustCompile(`^[A-Za-z0-9_.+-]{1,255}$`)
)

// ValidTagKey reports whether s may be served as the key of an instance tag.
func ValidTagKey(s string) bool { return tagKey.MatchString(s) }

// ValidTagValue reports whether s may be served as the value of an instance tag.
func ValidTagValue(s string) bool { return tagValue.MatchString(s) }
