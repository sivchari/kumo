package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// decodeDynamoDBJSON decodes raw, an AWS CLI-style DynamoDB JSON document
// (e.g. `{"pk":{"S":"a"},"n":{"N":"1"}}`), into a map of dynamodb
// AttributeValue members.
//
// This is hand-written, never generated: dynamodb's AttributeValue is a
// Smithy union (interface) type, so encoding/json cannot unmarshal directly
// into it the way cli-gen's generic FlagJSONBlob handling does for plain
// structs. cli-gen's FlagCustomFunc override (internal/cligen/overrides.go)
// routes every dynamodb Input field typed map[string]types.AttributeValue
// (Item, Key, ExpressionAttributeValues, ExclusiveStartKey) through this
// function instead.
func decodeDynamoDBJSON(raw string) (map[string]ddbtypes.AttributeValue, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("decode DynamoDB JSON: %w", err)
	}

	result := make(map[string]ddbtypes.AttributeValue, len(doc))

	for key, val := range doc {
		av, err := decodeAttributeValue(val)
		if err != nil {
			return nil, fmt.Errorf("decode DynamoDB JSON: attribute %q: %w", key, err)
		}

		result[key] = av
	}

	return result, nil
}

// decodeAttributeValue decodes raw (one DynamoDB JSON attribute value
// document, e.g. {"S":"a"} or {"L":[{"N":"1"},{"N":"2"}]}) into a dynamodb
// AttributeValue member, recursing through L (list) and M (map) members.
//
// It decodes into a map[string]json.RawMessage rather than a tagged struct
// so it can validate that exactly one AWS type-descriptor key (S, N, B,
// BOOL, NULL, SS, NS, BS, L, or M) is present before dispatching on it.
func decodeAttributeValue(raw json.RawMessage) (ddbtypes.AttributeValue, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, fmt.Errorf("unmarshal attribute value: %w", err)
	}

	if len(members) != 1 {
		return nil, fmt.Errorf("attribute value must set exactly one type (S, N, B, BOOL, NULL, SS, NS, BS, L, or M), got %d", len(members))
	}

	for kind, value := range members {
		decode, ok := attributeValueDecoders()[kind]
		if !ok {
			return nil, fmt.Errorf("unknown attribute value type %q", kind)
		}

		return decode(value)
	}

	panic("unreachable: members has exactly one entry")
}

// attributeValueDecoders returns a map from each AWS DynamoDB JSON
// type-descriptor key to the function that decodes its raw payload into the
// matching AttributeValue member. It is a function rather than a package
// variable because several of its entries (L, M) call back into
// decodeAttributeValue, which would otherwise create a package-level
// initialization cycle.
func attributeValueDecoders() map[string]func(json.RawMessage) (ddbtypes.AttributeValue, error) {
	return map[string]func(json.RawMessage) (ddbtypes.AttributeValue, error){
		"S":    decodeAttributeValueS,
		"N":    decodeAttributeValueN,
		"B":    decodeAttributeValueB,
		"BOOL": decodeAttributeValueBOOL,
		"NULL": decodeAttributeValueNULL,
		"SS":   decodeAttributeValueSS,
		"NS":   decodeAttributeValueNS,
		"BS":   decodeAttributeValueBS,
		"L":    decodeAttributeValueL,
		"M":    decodeAttributeValueM,
	}
}

// unmarshalMember unmarshals value (the raw JSON payload for AWS
// type-descriptor key kind) into a T, wrapping any error with kind for
// context.
func unmarshalMember[T any](kind string, value json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(value, &v); err != nil {
		return v, fmt.Errorf("decode %s: %w", kind, err)
	}

	return v, nil
}

func decodeAttributeValueS(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	s, err := unmarshalMember[string]("S", value)
	if err != nil {
		return nil, err
	}

	return &ddbtypes.AttributeValueMemberS{Value: s}, nil
}

func decodeAttributeValueN(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	n, err := unmarshalMember[string]("N", value)
	if err != nil {
		return nil, err
	}

	return &ddbtypes.AttributeValueMemberN{Value: n}, nil
}

func decodeAttributeValueB(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	encoded, err := unmarshalMember[string]("B", value)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode B: %w", err)
	}

	return &ddbtypes.AttributeValueMemberB{Value: decoded}, nil
}

func decodeAttributeValueBOOL(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	b, err := unmarshalMember[bool]("BOOL", value)
	if err != nil {
		return nil, err
	}

	return &ddbtypes.AttributeValueMemberBOOL{Value: b}, nil
}

func decodeAttributeValueNULL(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	b, err := unmarshalMember[bool]("NULL", value)
	if err != nil {
		return nil, err
	}

	return &ddbtypes.AttributeValueMemberNULL{Value: b}, nil
}

func decodeAttributeValueSS(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	ss, err := unmarshalMember[[]string]("SS", value)
	if err != nil {
		return nil, err
	}

	return &ddbtypes.AttributeValueMemberSS{Value: ss}, nil
}

func decodeAttributeValueNS(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	ns, err := unmarshalMember[[]string]("NS", value)
	if err != nil {
		return nil, err
	}

	return &ddbtypes.AttributeValueMemberNS{Value: ns}, nil
}

func decodeAttributeValueBS(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	encoded, err := unmarshalMember[[]string]("BS", value)
	if err != nil {
		return nil, err
	}

	decoded := make([][]byte, len(encoded))

	for i, s := range encoded {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("decode BS element %d: %w", i, err)
		}

		decoded[i] = b
	}

	return &ddbtypes.AttributeValueMemberBS{Value: decoded}, nil
}

func decodeAttributeValueL(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	items, err := unmarshalMember[[]json.RawMessage]("L", value)
	if err != nil {
		return nil, err
	}

	list := make([]ddbtypes.AttributeValue, len(items))

	for i, item := range items {
		av, err := decodeAttributeValue(item)
		if err != nil {
			return nil, fmt.Errorf("list element %d: %w", i, err)
		}

		list[i] = av
	}

	return &ddbtypes.AttributeValueMemberL{Value: list}, nil
}

func decodeAttributeValueM(value json.RawMessage) (ddbtypes.AttributeValue, error) {
	members, err := unmarshalMember[map[string]json.RawMessage]("M", value)
	if err != nil {
		return nil, err
	}

	m := make(map[string]ddbtypes.AttributeValue, len(members))

	for k, v := range members {
		av, err := decodeAttributeValue(v)
		if err != nil {
			return nil, fmt.Errorf("map member %q: %w", k, err)
		}

		m[k] = av
	}

	return &ddbtypes.AttributeValueMemberM{Value: m}, nil
}
