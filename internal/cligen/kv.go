package cligen

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// SetFieldsFromKV parses a "key=value,key2=value2" string and sets the
// matching exported fields of target (a struct or pointer-to-struct
// reflect.Value, addressable and settable). Keys are matched against each
// field's kebab-case flag name (see toKebabCase), case-insensitively, so
// "queue-url=..." matches a QueueUrl field.
//
// It is the single generic parser used both for flat-struct flags (FlagKV)
// and for each element of repeated flat-struct flags (FlagKVArray); it is
// also called from generated CLI code (cli/gen_*.go), which is why it is
// exported.
func SetFieldsFromKV(target reflect.Value, kv string) error {
	if target.Kind() == reflect.Pointer {
		if target.IsNil() {
			if !target.CanSet() {
				return fmt.Errorf("SetFieldsFromKV: nil pointer target is not settable")
			}

			target.Set(reflect.New(target.Type().Elem()))
		}

		target = target.Elem()
	}

	if target.Kind() != reflect.Struct {
		return fmt.Errorf("SetFieldsFromKV: target must be a struct, got %s", target.Kind())
	}

	for pair := range strings.SplitSeq(kv, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return fmt.Errorf("SetFieldsFromKV: invalid key=value pair %q", pair)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		field, err := findFieldByFlagKey(target, key)
		if err != nil {
			return err
		}

		if err := setScalarField(field, value); err != nil {
			return fmt.Errorf("SetFieldsFromKV: field %q: %w", key, err)
		}
	}

	return nil
}

func findFieldByFlagKey(target reflect.Value, key string) (reflect.Value, error) {
	t := target.Type()

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		if strings.EqualFold(toKebabCase(f.Name), key) {
			return target.Field(i), nil
		}
	}

	return reflect.Value{}, fmt.Errorf("SetFieldsFromKV: no field matching key %q on %s", key, t.Name())
}

func setScalarField(field reflect.Value, value string) error {
	if field.Kind() == reflect.Pointer {
		field.Set(reflect.New(field.Type().Elem()))
		field = field.Elem()
	}

	if field.Kind() == reflect.String {
		field.SetString(value)

		return nil
	}

	if field.Kind() == reflect.Int32 || field.Kind() == reflect.Int64 {
		return setIntField(field, value)
	}

	if field.Kind() == reflect.Bool {
		return setBoolField(field, value)
	}

	if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Uint8 {
		return setBytesField(field, value)
	}

	return fmt.Errorf("unsupported field kind %s", field.Type())
}

func setIntField(field reflect.Value, value string) error {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("parse int: %w", err)
	}

	field.SetInt(n)

	return nil
}

func setBoolField(field reflect.Value, value string) error {
	b, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("parse bool: %w", err)
	}

	field.SetBool(b)

	return nil
}

func setBytesField(field reflect.Value, value string) error {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}

	field.SetBytes(decoded)

	return nil
}
