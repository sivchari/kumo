package dynamodb

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const maxDigits38 = "9.9999999999999999999999999999999999999"

func TestValidateNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "integer", in: "42"},
		{name: "negative decimal", in: "-3.14"},
		{name: "zero", in: "0"},
		{name: "zero with huge exponent", in: "0e1000000000"},
		{name: "zero decimals", in: "0.000000"},
		{name: "largest magnitude", in: maxDigits38 + "E+125"},
		{name: "smallest magnitude", in: "1E-130"},
		{name: "smallest magnitude, shifted mantissa", in: "0.0001e-126"},
		{name: "38 digits", in: "12345678901234567890123456789012345678"},
		{name: "trailing zeros are not significant", in: "1" + strings.Repeat("0", 60) + "e-40"},
		{name: "leading zeros are not significant", in: "000" + strings.Repeat("0", 50) + "1"},
		{name: "not a number is left to the caller", in: "abc"},
		{name: "empty is left to the caller", in: ""},
		{name: "bare exponent is left to the caller", in: "e10"},
		{name: "overflow by one", in: "1E+126", want: msgNumberOverflow},
		{name: "overflow with huge exponent", in: "1e1000000000", want: msgNumberOverflow},
		{name: "overflow with exponent beyond int range", in: "1e" + strings.Repeat("9", 5000), want: msgNumberOverflow},
		{name: "overflow through a long integer part", in: strings.Repeat("9", 20) + strings.Repeat("0", 110) + "e0", want: msgNumberOverflow},
		{name: "underflow by one", in: "9E-131", want: msgNumberUnderflow},
		{name: "underflow with huge negative exponent", in: "1e-1000000000", want: msgNumberUnderflow},
		{name: "underflow through leading fraction zeros", in: "0." + strings.Repeat("0", 130) + "1", want: msgNumberUnderflow},
		{name: "39 digits", in: "123456789012345678901234567890123456789", want: msgNumberPrecision},
		{name: "39 digits after the point", in: "0.123456789012345678901234567890123456789", want: msgNumberPrecision},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateNumber(tt.in)

			if tt.want == "" {
				if err != nil {
					t.Fatalf("validateNumber(%.40q) = %v, want nil", tt.in, err)
				}

				return
			}

			var tErr *TableError
			if !errors.As(err, &tErr) {
				t.Fatalf("validateNumber(%.40q) = %v, want ValidationException", tt.in, err)
			}

			if tErr.Code != errCodeValidation || tErr.Message != tt.want {
				t.Errorf("validateNumber(%.40q) = %s: %q, want %s: %q", tt.in, tErr.Code, tErr.Message, errCodeValidation, tt.want)
			}
		})
	}
}

func TestParseNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{name: "integer", in: "42", want: "42", wantOK: true},
		{name: "decimal", in: "0.5", want: "1/2", wantOK: true},
		{name: "exponent", in: "1.5e2", want: "150", wantOK: true},
		{name: "zero with huge exponent", in: "0e1000000000", want: "0", wantOK: true},
		{name: "huge exponent", in: "1e1000000000"},
		{name: "tiny exponent", in: "1e-1000000000"},
		{name: "too many digits", in: "123456789012345678901234567890123456789"},
		{name: "fraction is not a DynamoDB number", in: "1/2"},
		{name: "not a number", in: "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, ok := parseNumber(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("parseNumber(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}

			if ok && r.RatString() != tt.want {
				t.Errorf("parseNumber(%q) = %s, want %s", tt.in, r.RatString(), tt.want)
			}
		})
	}
}

// TestNumberComparisonBoundsWork asserts that an absurd exponent is rejected
// before big.Rat expands it, by requiring the comparison to return promptly.
func TestNumberComparisonBoundsWork(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})

	go func() {
		defer close(done)

		compareNumbers("1e1000000000", "1")
		numberKey("1e1000000000")
		compareKeyValues(&AttributeValue{N: new("1e1000000000")}, &AttributeValue{N: new("2")})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("comparing an out-of-range number did not return promptly")
	}
}

func TestCompareNumbers_OutOfRange(t *testing.T) {
	t.Parallel()

	if _, ok := compareNumbers("1e1000000000", "1"); ok {
		t.Error("compareNumbers accepted an out-of-range number")
	}

	if order, ok := compareNumbers(maxDigits38+"E+125", "1"); !ok || order <= 0 {
		t.Errorf("compareNumbers(max, 1) = %d, %v, want a positive order", order, ok)
	}
}

func TestAttributeValueUnmarshal_NumberRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "N in range", json: `{"N":"1e100"}`},
		{name: "N overflow", json: `{"N":"1e1000000000"}`, want: msgNumberOverflow},
		{name: "NS underflow", json: `{"NS":["1","1e-131"]}`, want: msgNumberUnderflow},
		{name: "nested in a map", json: `{"M":{"a":{"L":[{"N":"1e126"}]}}}`, want: msgNumberOverflow},
		{name: "precision", json: `{"N":"123456789012345678901234567890123456789"}`, want: msgNumberPrecision},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var av AttributeValue

			err := json.Unmarshal([]byte(tt.json), &av)

			if tt.want == "" {
				if err != nil {
					t.Fatalf("Unmarshal = %v, want nil", err)
				}

				return
			}

			var tErr *TableError
			if !errors.As(err, &tErr) || tErr.Message != tt.want || tErr.Code != errCodeValidation {
				t.Fatalf("Unmarshal = %v, want ValidationException %q", err, tt.want)
			}
		})
	}
}
