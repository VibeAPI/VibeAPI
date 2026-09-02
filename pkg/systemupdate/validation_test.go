package systemupdate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrictUpdateIdentifiers(t *testing.T) {
	assert.True(t, ValidReleaseID("v4.10.14"))
	assert.False(t, ValidReleaseID("../latest"))
	assert.False(t, ValidReleaseID("repo:tag"))
	assert.True(t, ValidDigest("sha256:"+strings.Repeat("a", 64)))
	assert.False(t, ValidDigest("sha256:not-a-digest"))
	assert.True(t, ValidIdempotencyKey("update:20260901:abcdef"))
	assert.False(t, ValidIdempotencyKey("short"))
}
