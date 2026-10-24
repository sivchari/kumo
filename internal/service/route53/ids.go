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
	// in a byte: 252, or 7*36. drawID keeps a random byte b only below it and
	// maps b to idAlphabet[b%len(idAlphabet)], so each character comes from
	// exactly 7 of the 252 kept values and all characters are equally likely.
	// The IDs are therefore free of modulo bias by construction (rejection
	// sampling). Keeping 252-255 as well would map them to A-D and make each
	// of those 8/256 likely against 7/256 for the other characters.
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
// idAlphabet, drawn from crypto/rand.
func newID(prefix byte) string {
	return drawID(prefix, fillRandom)
}

// fillRandom fills b from crypto/rand, whose Read never returns an error and
// always fills b entirely.
func fillRandom(b []byte) {
	_, _ = rand.Read(b)
}

// drawID returns prefix followed by idRandomLength characters from
// idAlphabet, made from the bytes fill writes, skipping every byte at or
// above idRejectThreshold. Tests pass their own fill to control the bytes.
func drawID(prefix byte, fill func([]byte)) string {
	id := make([]byte, 1, 1+idRandomLength)
	id[0] = prefix

	buf := make([]byte, idRandomLength*2)
	for len(id) < cap(id) {
		fill(buf)

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
