package systemupdate

import "regexp"

var (
	releaseIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	digestPattern      = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{7,127}$`)
)

func ValidReleaseID(value string) bool      { return releaseIDPattern.MatchString(value) }
func ValidDigest(value string) bool         { return digestPattern.MatchString(value) }
func ValidIdempotencyKey(value string) bool { return idempotencyPattern.MatchString(value) }
