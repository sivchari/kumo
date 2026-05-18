package dynamodb

import (
	"fmt"
	"strconv"
	"strings"
)

// ConditionInput holds the parameters for evaluating a condition expression.
type ConditionInput struct {
	Expression string
	ExprNames  map[string]string
	ExprValues map[string]AttributeValue
}

// evaluateCondition evaluates a condition expression against an item.
// Returns true if the condition is satisfied or the expression is empty.
func evaluateCondition(item Item, cond ConditionInput) (bool, error) {
	if cond.Expression == "" {
		return true, nil
	}

	expr := resolveNames(cond.Expression, cond.ExprNames)

	result, _, err := parseOrExpr(expr, item, cond.ExprValues)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate condition: %w", err)
	}

	return result, nil
}

// resolveNames replaces expression attribute name placeholders with actual names.
func resolveNames(expr string, names map[string]string) string {
	for placeholder, name := range names {
		expr = strings.ReplaceAll(expr, placeholder, name)
	}

	return expr
}

// parseOrExpr parses an OR expression: expr OR expr OR ...
func parseOrExpr(expr string, item Item, values map[string]AttributeValue) (bool, string, error) {
	result, rest, err := parseAndExpr(expr, item, values)
	if err != nil {
		return false, "", err
	}

	for {
		rest = strings.TrimSpace(rest)
		if !startsWithKeyword(rest, "OR") {
			return result, rest, nil
		}

		rest = strings.TrimSpace(rest[2:])

		right, newRest, err := parseAndExpr(rest, item, values)
		if err != nil {
			return false, "", err
		}

		result = result || right
		rest = newRest
	}
}

// parseAndExpr parses an AND expression: expr AND expr AND ...
func parseAndExpr(expr string, item Item, values map[string]AttributeValue) (bool, string, error) {
	result, rest, err := parseNotExpr(expr, item, values)
	if err != nil {
		return false, "", err
	}

	for {
		rest = strings.TrimSpace(rest)
		if !startsWithKeyword(rest, "AND") {
			return result, rest, nil
		}

		rest = strings.TrimSpace(rest[3:])

		right, newRest, err := parseNotExpr(rest, item, values)
		if err != nil {
			return false, "", err
		}

		result = result && right
		rest = newRest
	}
}

// parseNotExpr parses a NOT expression or delegates to primary.
func parseNotExpr(expr string, item Item, values map[string]AttributeValue) (bool, string, error) {
	trimmed := strings.TrimSpace(expr)
	if startsWithKeyword(trimmed, "NOT") {
		rest := strings.TrimSpace(trimmed[3:])

		result, newRest, err := parsePrimary(rest, item, values)
		if err != nil {
			return false, "", err
		}

		return !result, newRest, nil
	}

	return parsePrimary(trimmed, item, values)
}

// parsePrimary parses a primary expression: parenthesized, function call, or comparison.
func parsePrimary(expr string, item Item, values map[string]AttributeValue) (bool, string, error) {
	trimmed := strings.TrimSpace(expr)

	// Parenthesized expression.
	if strings.HasPrefix(trimmed, "(") {
		inner := trimmed[1:]

		result, rest, err := parseOrExpr(inner, item, values)
		if err != nil {
			return false, "", err
		}

		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, ")") {
			return false, "", fmt.Errorf("expected closing parenthesis")
		}

		return result, rest[1:], nil
	}

	// Function calls.
	for _, fn := range []string{"attribute_exists", "attribute_not_exists", "begins_with", "contains"} {
		if strings.HasPrefix(trimmed, fn+"(") {
			return parseFunctionCall(fn, trimmed[len(fn):], item, values)
		}

		// Handle optional space between function name and parenthesis: fn (args).
		if strings.HasPrefix(trimmed, fn+" (") {
			return parseFunctionCall(fn, strings.TrimSpace(trimmed[len(fn):]), item, values)
		}
	}

	// size() function used in comparison: size(path) op value
	if strings.HasPrefix(trimmed, "size(") {
		return parseSizeComparison(trimmed, item, values)
	}

	// Comparison: operand op operand
	return parseComparison(trimmed, item, values)
}

// parseFunctionCall parses and evaluates a function call.
func parseFunctionCall(fn, argsStr string, item Item, values map[string]AttributeValue) (bool, string, error) {
	args, rest, err := parseArgList(argsStr)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse %s arguments: %w", fn, err)
	}

	switch fn {
	case "attribute_exists":
		if len(args) != 1 {
			return false, "", fmt.Errorf("attribute_exists requires 1 argument")
		}

		path := strings.TrimSpace(args[0])
		_, exists := resolveItemPath(item, path)

		return exists, rest, nil

	case "attribute_not_exists":
		if len(args) != 1 {
			return false, "", fmt.Errorf("attribute_not_exists requires 1 argument")
		}

		path := strings.TrimSpace(args[0])
		_, exists := resolveItemPath(item, path)

		return !exists, rest, nil

	case "begins_with":
		if len(args) != 2 {
			return false, "", fmt.Errorf("begins_with requires 2 arguments")
		}

		path := strings.TrimSpace(args[0])
		val := resolveOperand(strings.TrimSpace(args[1]), item, values)

		av, exists := resolveItemPath(item, path)
		if !exists || av.S == nil || val.S == nil {
			return false, rest, nil
		}

		return strings.HasPrefix(*av.S, *val.S), rest, nil

	case "contains":
		if len(args) != 2 {
			return false, "", fmt.Errorf("contains requires 2 arguments")
		}

		path := strings.TrimSpace(args[0])
		val := resolveOperand(strings.TrimSpace(args[1]), item, values)

		av, exists := resolveItemPath(item, path)
		if !exists {
			return false, rest, nil
		}

		return evalContains(av, val), rest, nil

	default:
		return false, "", fmt.Errorf("unknown function: %s", fn)
	}
}

// evalContains evaluates the contains function for various types.
//
//nolint:gocritic // hugeParam: AttributeValue passed by value intentionally.
func evalContains(av AttributeValue, operand AttributeValue) bool {
	// String contains substring.
	if av.S != nil && operand.S != nil {
		return strings.Contains(*av.S, *operand.S)
	}

	// String set contains value.
	if av.SS != nil && operand.S != nil {
		for _, s := range av.SS {
			if s == *operand.S {
				return true
			}
		}

		return false
	}

	// Number set contains value.
	if av.NS != nil && operand.N != nil {
		for _, n := range av.NS {
			if n == *operand.N {
				return true
			}
		}

		return false
	}

	// List contains value.
	if av.L != nil {
		for _, elem := range av.L {
			if elem == nil {
				continue
			}

			if attributeValuesEqualStatic(*elem, operand) {
				return true
			}
		}

		return false
	}

	return false
}

// parseSizeComparison parses size(path) op value.
func parseSizeComparison(expr string, item Item, values map[string]AttributeValue) (bool, string, error) {
	// Extract path from size(...).
	inner := expr[5:] // skip "size("

	parenDepth := 1
	idx := 0

	for idx < len(inner) && parenDepth > 0 {
		switch inner[idx] {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		}

		if parenDepth > 0 {
			idx++
		}
	}

	if parenDepth != 0 {
		return false, "", fmt.Errorf("unmatched parenthesis in size()")
	}

	path := strings.TrimSpace(inner[:idx])
	rest := strings.TrimSpace(inner[idx+1:])

	// Get the size of the attribute.
	av, exists := resolveItemPath(item, path)
	if !exists {
		return false, "", fmt.Errorf("attribute %s not found for size()", path)
	}

	sizeVal := attributeSize(av)

	// Parse operator.
	op, afterOp, err := parseComparisonOp(rest)
	if err != nil {
		return false, "", err
	}

	// Parse right operand.
	rightToken, finalRest := nextToken(strings.TrimSpace(afterOp))
	rightVal := resolveOperand(rightToken, item, values)

	if rightVal.N == nil {
		return false, "", fmt.Errorf("size() comparison requires numeric operand")
	}

	rightNum, err := strconv.ParseFloat(*rightVal.N, 64)
	if err != nil {
		return false, "", fmt.Errorf("invalid number: %s", *rightVal.N)
	}

	result := compareNumbers(float64(sizeVal), rightNum, op)

	return result, finalRest, nil
}

// attributeSize returns the size of an attribute value.
//
//nolint:gocritic // hugeParam: AttributeValue passed by value intentionally.
func attributeSize(av AttributeValue) int {
	switch {
	case av.S != nil:
		return len(*av.S)
	case av.N != nil:
		return len(*av.N)
	case av.B != nil:
		return len(av.B)
	case av.SS != nil:
		return len(av.SS)
	case av.NS != nil:
		return len(av.NS)
	case av.BS != nil:
		return len(av.BS)
	case av.L != nil:
		return len(av.L)
	case av.M != nil:
		return len(av.M)
	default:
		return 0
	}
}

// parseComparison parses a comparison expression: operand op operand.
func parseComparison(expr string, item Item, values map[string]AttributeValue) (bool, string, error) {
	leftToken, rest := nextToken(strings.TrimSpace(expr))
	if leftToken == "" {
		return false, "", fmt.Errorf("expected operand")
	}

	rest = strings.TrimSpace(rest)

	// Handle BETWEEN: operand BETWEEN operand AND operand
	if startsWithKeyword(rest, "BETWEEN") {
		return parseBetween(leftToken, strings.TrimSpace(rest[7:]), item, values)
	}

	op, afterOp, err := parseComparisonOp(rest)
	if err != nil {
		return false, "", err
	}

	rightToken, finalRest := nextToken(strings.TrimSpace(afterOp))
	if rightToken == "" {
		return false, "", fmt.Errorf("expected right operand")
	}

	left := resolveOperand(leftToken, item, values)
	right := resolveOperand(rightToken, item, values)

	result := compareAttributeValues(left, right, op)

	return result, finalRest, nil
}

// parseBetween handles "operand BETWEEN lo AND hi".
func parseBetween(leftToken, rest string, item Item, values map[string]AttributeValue) (bool, string, error) {
	loToken, rest := nextToken(strings.TrimSpace(rest))
	if loToken == "" {
		return false, "", fmt.Errorf("expected low operand in BETWEEN")
	}

	rest = strings.TrimSpace(rest)
	if !startsWithKeyword(rest, "AND") {
		return false, "", fmt.Errorf("expected AND in BETWEEN expression")
	}

	rest = strings.TrimSpace(rest[3:])

	hiToken, finalRest := nextToken(rest)
	if hiToken == "" {
		return false, "", fmt.Errorf("expected high operand in BETWEEN")
	}

	val := resolveOperand(leftToken, item, values)
	lo := resolveOperand(loToken, item, values)
	hi := resolveOperand(hiToken, item, values)

	// val >= lo AND val <= hi
	result := compareAttributeValues(val, lo, ">=") && compareAttributeValues(val, hi, "<=")

	return result, finalRest, nil
}

// parseComparisonOp extracts a comparison operator from the front of the string.
func parseComparisonOp(s string) (string, string, error) {
	if strings.HasPrefix(s, "<>") {
		return "<>", s[2:], nil
	}

	if strings.HasPrefix(s, "<=") {
		return "<=", s[2:], nil
	}

	if strings.HasPrefix(s, ">=") {
		return ">=", s[2:], nil
	}

	if strings.HasPrefix(s, "=") {
		return "=", s[1:], nil
	}

	if strings.HasPrefix(s, "<") {
		return "<", s[1:], nil
	}

	if strings.HasPrefix(s, ">") {
		return ">", s[1:], nil
	}

	return "", "", fmt.Errorf("expected comparison operator, got: %.20s", s)
}

// nextToken extracts the next token from the string.
// A token is a contiguous sequence of non-whitespace, non-operator characters,
// or a value placeholder starting with ':'.
func nextToken(s string) (string, string) {
	if s == "" {
		return "", ""
	}

	i := 0
	for i < len(s) {
		ch := s[i]
		if ch == ' ' || ch == '\t' || ch == ')' {
			break
		}

		if i > 0 && isOperatorStart(s[i:]) {
			break
		}

		i++
	}

	return s[:i], s[i:]
}

// isOperatorStart checks if the string starts with a comparison operator or keyword.
func isOperatorStart(s string) bool {
	if s == "" {
		return false
	}

	switch s[0] {
	case '=', '<', '>':
		return true
	default:
		return false
	}
}

// resolveOperand resolves an operand token to an AttributeValue.
// It can be a value placeholder (:val), or an attribute path.
func resolveOperand(token string, item Item, values map[string]AttributeValue) AttributeValue {
	if strings.HasPrefix(token, ":") {
		if val, ok := values[token]; ok {
			return val
		}

		return AttributeValue{}
	}

	av, _ := resolveItemPath(item, token)

	return av
}

// resolveItemPath resolves a dotted path on an item, returning the value and whether it exists.
func resolveItemPath(item Item, path string) (AttributeValue, bool) {
	parts := strings.Split(path, ".")

	if len(parts) == 1 {
		val, ok := item[path]

		return val, ok
	}

	// Nested path traversal.
	val, ok := item[parts[0]]
	if !ok {
		return AttributeValue{}, false
	}

	for _, part := range parts[1:] {
		if val.M == nil {
			return AttributeValue{}, false
		}

		ptr, ok := val.M[part]
		if !ok || ptr == nil {
			return AttributeValue{}, false
		}

		val = *ptr
	}

	return val, true
}

// compareAttributeValues compares two attribute values using the given operator.
//
//nolint:gocritic,cyclop // hugeParam: AttributeValue passed by value for comparison.
func compareAttributeValues(a, b AttributeValue, op string) bool {
	// String comparison.
	if a.S != nil && b.S != nil {
		return compareStrings(*a.S, *b.S, op)
	}

	// Number comparison.
	if a.N != nil && b.N != nil {
		aNum, err1 := strconv.ParseFloat(*a.N, 64)
		bNum, err2 := strconv.ParseFloat(*b.N, 64)

		if err1 != nil || err2 != nil {
			return false
		}

		return compareNumbers(aNum, bNum, op)
	}

	// Boolean comparison (only = and <>).
	if a.BOOL != nil && b.BOOL != nil {
		switch op {
		case "=":
			return *a.BOOL == *b.BOOL
		case "<>":
			return *a.BOOL != *b.BOOL
		default:
			return false
		}
	}

	// NULL comparison (only = and <>).
	if a.NULL != nil && b.NULL != nil {
		switch op {
		case "=":
			return *a.NULL == *b.NULL
		case "<>":
			return *a.NULL != *b.NULL
		default:
			return false
		}
	}

	// Type mismatch or unsupported types: only <> returns true.
	return op == "<>"
}

func compareStrings(a, b, op string) bool {
	switch op {
	case "=":
		return a == b
	case "<>":
		return a != b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	default:
		return false
	}
}

func compareNumbers(a, b float64, op string) bool {
	switch op {
	case "=":
		return a == b
	case "<>":
		return a != b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	default:
		return false
	}
}

// attributeValuesEqualStatic compares two attribute values for equality (static function).
//
//nolint:gocritic // hugeParam: AttributeValue passed by value for comparison.
func attributeValuesEqualStatic(a, b AttributeValue) bool {
	if a.S != nil && b.S != nil {
		return *a.S == *b.S
	}

	if a.N != nil && b.N != nil {
		return *a.N == *b.N
	}

	if a.BOOL != nil && b.BOOL != nil {
		return *a.BOOL == *b.BOOL
	}

	if a.NULL != nil && b.NULL != nil {
		return *a.NULL == *b.NULL
	}

	return false
}

// parseArgList parses a parenthesized, comma-separated argument list.
// Input should start with "(" and returns the arguments and remaining string after ")".
func parseArgList(s string) ([]string, string, error) {
	if !strings.HasPrefix(s, "(") {
		return nil, "", fmt.Errorf("expected '('")
	}

	s = s[1:] // skip '('

	var args []string

	depth := 1
	start := 0

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				arg := strings.TrimSpace(s[start:i])
				if arg != "" {
					args = append(args, arg)
				}

				return args, s[i+1:], nil
			}
		case ',':
			if depth == 1 {
				args = append(args, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}

	return nil, "", fmt.Errorf("unmatched parenthesis")
}

// startsWithKeyword checks if s starts with the given keyword followed by a space or end of string.
func startsWithKeyword(s, keyword string) bool {
	if !strings.HasPrefix(strings.ToUpper(s), keyword) {
		return false
	}

	if len(s) == len(keyword) {
		return true
	}

	next := s[len(keyword)]

	return next == ' ' || next == '\t' || next == '('
}
