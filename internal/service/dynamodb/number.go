package dynamodb

import (
	"math/big"
)

// DynamoDB number limits: up to 38 significant digits, with a magnitude from
// 1E-130 to 9.9999999999999999999999999999999999999E+125.
const (
	maxNumberDigits   = 38
	maxNumberExponent = 125
	minNumberExponent = -130

	// exponentSaturation caps how far an exponent is accumulated. Any exponent
	// beyond it is out of range anyway, and the cap keeps the arithmetic from
	// overflowing on an arbitrarily long run of exponent digits.
	exponentSaturation = 1 << 30
)

const (
	msgNumberOverflow  = "Number overflow. Attempting to store a number with magnitude larger than supported range"
	msgNumberUnderflow = "Number underflow. Attempting to store a number with magnitude smaller than supported range"
	msgNumberPrecision = "Attempting to store more than 38 significant digits in a Number"
)

// numberShape describes a decimal number string without materializing it.
type numberShape struct {
	// digits is the count of significant digits, ignoring leading and
	// trailing zeros. It is zero for the number zero.
	digits int
	// exponent is the decimal exponent of the most significant digit, as in
	// scientific notation. It is meaningless for the number zero.
	exponent int
}

// scanNumber reports the shape of s without allocating, and ok false when s
// is not a plain decimal number with an optional exponent.
func scanNumber(s string) (shape numberShape, ok bool) {
	shape, rest, ok := scanMantissa(s)
	if !ok {
		return numberShape{}, false
	}

	exp, ok := scanExponent(rest)
	if !ok {
		return numberShape{}, false
	}

	shape.exponent += exp

	return shape, true
}

// scanMantissa reads the sign and decimal digits at the start of s and
// returns the remainder. The returned exponent is that of the mantissa alone.
func scanMantissa(s string) (shape numberShape, rest string, ok bool) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}

	var (
		seenDigit   bool
		seenDot     bool
		significant bool
		// intDigits counts the digits before the decimal point and
		// leadingZeros the zero digits before the first significant one;
		// together they place the decimal point relative to it.
		intDigits    int
		leadingZeros int
		trailing     int
	)

	for ; i < len(s); i++ {
		c := s[i]

		if c == '.' {
			if seenDot {
				return numberShape{}, "", false
			}

			seenDot = true

			continue
		}

		if c < '0' || c > '9' {
			break
		}

		seenDigit = true

		if !seenDot {
			intDigits++
		}

		switch {
		case c != '0':
			significant = true
			shape.digits += trailing + 1
			trailing = 0
		case significant:
			trailing++
		default:
			leadingZeros++
		}
	}

	if !seenDigit {
		return numberShape{}, "", false
	}

	shape.exponent = intDigits - leadingZeros - 1

	return shape, s[i:], true
}

// scanExponent parses an optional exponent suffix (e or E, optional sign,
// digits) that must span all of rest, saturating instead of overflowing.
func scanExponent(rest string) (exp int, ok bool) {
	if rest == "" {
		return 0, true
	}

	if rest[0] != 'e' && rest[0] != 'E' {
		return 0, false
	}

	rest = rest[1:]

	negative := false

	if rest != "" && (rest[0] == '+' || rest[0] == '-') {
		negative = rest[0] == '-'
		rest = rest[1:]
	}

	if rest == "" {
		return 0, false
	}

	for i := range len(rest) {
		c := rest[i]
		if c < '0' || c > '9' {
			return 0, false
		}

		if exp < exponentSaturation {
			exp = exp*10 + int(c-'0')
		}
	}

	if negative {
		exp = -exp
	}

	return exp, true
}

// validateNumber rejects a well-formed number that DynamoDB cannot store:
// one with more than 38 significant digits or outside the supported
// magnitude. A string that is not a number is left for the caller to handle.
func validateNumber(s string) error {
	shape, ok := scanNumber(s)
	if !ok || shape.digits == 0 {
		return nil
	}

	switch {
	case shape.exponent > maxNumberExponent:
		return newValidationException(msgNumberOverflow)
	case shape.exponent < minNumberExponent:
		return newValidationException(msgNumberUnderflow)
	case shape.digits > maxNumberDigits:
		return newValidationException(msgNumberPrecision)
	default:
		return nil
	}
}

// parseNumber parses s as an exact number. ok is false if s is not a number
// or is outside DynamoDB's range, which also keeps big.Rat from allocating
// for a huge exponent such as 1e1000000000.
func parseNumber(s string) (r *big.Rat, ok bool) {
	shape, scanned := scanNumber(s)
	if !scanned || validateNumber(s) != nil {
		return nil, false
	}

	if shape.digits == 0 {
		return new(big.Rat), true
	}

	return new(big.Rat).SetString(s)
}
