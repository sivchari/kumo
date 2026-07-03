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

	// Function calls. attribute_type belongs here: like the others it takes a
	// parenthesized argument list and returns a boolean. (size() is handled
	// separately below because its syntax is "size(path) op value".)
	for _, fn := range []string{"attribute_exists", "attribute_not_exists", "attribute_type", "begins_with", "contains"} {
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

// matchesAttributeType reports whether the attribute value is of the given
// DynamoDB type descriptor.
//
//nolint:gocritic // hugeParam: AttributeValue passed by value intentionally.
func matchesAttributeType(av AttributeValue, typ string) bool {
	switch typ {
	case "S":
		return av.S != nil
	case "N":
		return av.N != nil
	case "B":
		return av.B != nil
	case "SS":
		return av.SS != nil
	case "NS":
		return av.NS != nil
	case "BS":
		return av.BS != nil
	case "M":
		return av.M != nil
	case "L":
		return av.L != nil
	case "NULL":
		return av.NULL != nil
	case "BOOL":
		return av.BOOL != nil
	default:
		return false
	}
}

// parseFunctionCall parses a function's argument list and dispatches to the
// matching evaluator. The remaining input after the argument list is the same
// for every function, so it is returned here rather than by each evaluator.
func parseFunctionCall(fn, argsStr string, item Item, values map[string]AttributeValue) (bool, string, error) {
	args, rest, err := parseArgList(argsStr)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse %s arguments: %w", fn, err)
	}

	var result bool

	switch fn {
	case "attribute_exists":
		result, err = evalAttributeExists(args, item)
	case "attribute_not_exists":
		result, err = evalAttributeNotExists(args, item)
	case "attribute_type":
		result, err = evalAttributeType(args, item, values)
	case "begins_with":
		result, err = evalBeginsWith(args, item, values)
	case "contains":
		result, err = evalContainsFunc(args, item, values)
	default:
		return false, "", fmt.Errorf("unknown function: %s", fn)
	}

	if err != nil {
		return false, "", err
	}

	return result, rest, nil
}

func evalAttributeExists(args []string, item Item) (bool, error) {
	if len(args) != 1 {
		return false, fmt.Errorf("attribute_exists requires 1 argument")
	}

	_, exists := resolveItemPath(item, strings.TrimSpace(args[0]))

	return exists, nil
}

func evalAttributeNotExists(args []string, item Item) (bool, error) {
	if len(args) != 1 {
		return false, fmt.Errorf("attribute_not_exists requires 1 argument")
	}

	_, exists := resolveItemPath(item, strings.TrimSpace(args[0]))

	return !exists, nil
}

// evalAttributeType evaluates attribute_type(path, :type). Per DynamoDB the type
// argument must be an expression attribute value holding one of the type
// descriptors (S, N, B, SS, NS, BS, M, L, NULL, BOOL).
func evalAttributeType(args []string, item Item, values map[string]AttributeValue) (bool, error) {
	if len(args) != 2 {
		return false, fmt.Errorf("attribute_type requires 2 arguments")
	}

	typeVal := resolveOperand(strings.TrimSpace(args[1]), item, values)
	if typeVal.S == nil {
		return false, fmt.Errorf("attribute_type requires a string type operand")
	}

	av, exists := resolveItemPath(item, strings.TrimSpace(args[0]))
	if !exists {
		return false, nil
	}

	return matchesAttributeType(av, *typeVal.S), nil
}

func evalBeginsWith(args []string, item Item, values map[string]AttributeValue) (bool, error) {
	if len(args) != 2 {
		return false, fmt.Errorf("begins_with requires 2 arguments")
	}

	val := resolveOperand(strings.TrimSpace(args[1]), item, values)

	av, exists := resolveItemPath(item, strings.TrimSpace(args[0]))
	if !exists || av.S == nil || val.S == nil {
		return false, nil
	}

	return strings.HasPrefix(*av.S, *val.S), nil
}

func evalContainsFunc(args []string, item Item, values map[string]AttributeValue) (bool, error) {
	if len(args) != 2 {
		return false, fmt.Errorf("contains requires 2 arguments")
	}

	val := resolveOperand(strings.TrimSpace(args[1]), item, values)

	av, exists := resolveItemPath(item, strings.TrimSpace(args[0]))
	if !exists {
		return false, nil
	}

	return evalContains(av, val), nil
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

	// Parse operator and right operand first so the remaining string (finalRest)
	// is consumed correctly even when the attribute is absent.
	op, afterOp, err := parseComparisonOp(rest)
	if err != nil {
		return false, "", err
	}

	rightToken, finalRest := nextToken(strings.TrimSpace(afterOp))

	rightVal := resolveOperand(rightToken, item, values)
	if rightVal.N == nil {
		return false, "", fmt.Errorf("size() comparison requires numeric operand")
	}

	rightNum, err := strconv.ParseFloat(*rightVal.N, 64)
	if err != nil {
		return false, "", fmt.Errorf("invalid number: %s", *rightVal.N)
	}

	// A missing attribute is not an error: like any other comparison against an
	// absent attribute, the item simply does not match (DynamoDB does not fail).
	av, exists := resolveItemPath(item, path)
	if !exists {
		return false, finalRest, nil
	}

	result := compareNumbers(float64(attributeSize(av)), rightNum, op)

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

	// Handle IN: operand IN (operand, operand, ...)
	if startsWithKeyword(rest, "IN") {
		return parseIn(leftToken, strings.TrimSpace(rest[2:]), item, values)
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

// maxInOperands is DynamoDB's limit on the number of operands in an IN list.
const maxInOperands = 100

// parseIn handles "operand IN (operand, operand, ...)". The result is true if
// the left operand equals any operand in the list.
func parseIn(leftToken, rest string, item Item, values map[string]AttributeValue) (bool, string, error) {
	args, finalRest, err := parseArgList(rest)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse IN operand list: %w", err)
	}

	if len(args) == 0 {
		return false, "", fmt.Errorf("IN requires at least 1 operand")
	}

	if len(args) > maxInOperands {
		return false, "", fmt.Errorf("IN accepts at most %d operands, got %d", maxInOperands, len(args))
	}

	val := resolveOperand(leftToken, item, values)

	result := false

	for _, arg := range args {
		operand := resolveOperand(strings.TrimSpace(arg), item, values)
		if compareAttributeValues(val, operand, "=") {
			result = true

			break
		}
	}

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
// equalityOnly evaluates the only operators DynamoDB allows for BOOL and NULL
// operands: equality and inequality. Any other operator is false.
func equalityOnly(equal bool, op string) bool {
	switch op {
	case "=":
		return equal
	case "<>":
		return !equal
	default:
		return false
	}
}

//nolint:gocritic // hugeParam: AttributeValue passed by value for comparison.
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
		return equalityOnly(*a.BOOL == *b.BOOL, op)
	}

	// NULL comparison (only = and <>).
	if a.NULL != nil && b.NULL != nil {
		return equalityOnly(*a.NULL == *b.NULL, op)
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
