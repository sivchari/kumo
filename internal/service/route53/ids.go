package route53

import (
	"crypto/rand"
	"errors"
	"fmt"
)

const (
	// idAlphabet is the character set of the IDs in the Route 53 API
	// reference examples, such as /hostedzone/Z1PA6795UKMFR9.
	idAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// idRandomLength is the number of random characters after the one-letter
	// prefix. The Route 53 API reference limits HostedZone.Id,
	// AliasTarget.HostedZoneId and ChangeInfo.Id to 32 characters; the
	// resulting 14-character ID fits that limit even with its "/hostedzone/"
	// or "/change/" path prefix.
	idRandomLength = 13
	// idRejectThreshold is the largest multiple of len(idAlphabet) that fits
	// in a byte; random bytes at or above it are discarded to avoid modulo bias.
	idRejectThreshold = 256 - 256%len(idAlphabet)
	// maxIDAttempts bounds how many generated IDs insertWithNewID tries. With
	// 36^13 possible IDs even one collision is unlikely; the bound only turns
	// a generator that keeps returning taken IDs into an error instead of an
	// endless loop.
	maxIDAttempts = 5
)

// errNoFreeID is returned when every generated ID was already taken.
var errNoFreeID = errors.New("every generated ID was already taken")

// insertWithNewID calls insert with IDs from next until insert returns
// something other than taken, the error storage reports for an ID that is
// already in use, and returns that result. It gives up after maxIDAttempts.
func insertWithNewID(next func() string, taken error, insert func(id string) error) error {
	for range maxIDAttempts {
		if err := insert(next()); !errors.Is(err, taken) {
			return err
		}
	}

	return fmt.Errorf("%w after %d attempts", errNoFreeID, maxIDAttempts)
}

// newHostedZoneID returns a bare hosted zone ID such as Z1PA6795UKMFR9.
func newHostedZoneID() string {
	return newID('Z')
}

// newChangeID returns a bare change ID such as C1PA6795UKMFR9.
func newChangeID() string {
	return newID('C')
}

// newID returns prefix followed by idRandomLength random characters from
// idAlphabet.
func newID(prefix byte) string {
	id := make([]byte, 1, 1+idRandomLength)
	id[0] = prefix

	buf := make([]byte, idRandomLength*2)
	for len(id) < cap(id) {
		// crypto/rand.Read never returns an error.
		_, _ = rand.Read(buf)

		for _, b := range buf {
			if int(b) >= idRejectThreshold {
				continue
			}

			id = append(id, idAlphabet[int(b)%len(idAlphabet)])
			if len(id) == cap(id) {
				break
			}
		}
	}

	return string(id)
}
