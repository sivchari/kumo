package dynamodb

import (
	"fmt"
	"strings"
)

const (
	tokenEOF       = "<EOF>"
	tokenComma     = ","
	tokenAssign    = "="
	tokenPlus      = "+"
	tokenMinus     = "-"
	tokenOpenParen = "("
	tokenCloseP    = ")"

	updateExpressionPrefix = "Invalid UpdateExpression: "
)

// updateClause is one SET, ADD, DELETE or REMOVE section of an update
// expression. body holds the raw text after the keyword, with expression
// attribute name placeholders still unresolved.
type updateClause struct {
	action string // SET, ADD, DELETE, REMOVE
	body   string
}

// exprToken is a lexical token of an update expression with its byte range in
// the source.
type exprToken struct {
	text       string
	start, end int
	word       bool // false for the single-character delimiters
}

// isExpressionSpace reports whether b separates the parts of an update
// expression. The AWS SDK's expression builder joins clauses with a newline, so
// a clause keyword is not always surrounded by spaces.
func isExpressionSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// isExpressionDelimiter reports whether b is a single-character token.
func isExpressionDelimiter(b byte) bool {
	switch b {
	case ',', '=', '+', '-', '(', ')':
		return true
	default:
		return false
	}
}

func asciiUpper(s string) string {
	out := []byte(s)
	for i, b := range out {
		if b >= 'a' && b <= 'z' {
			out[i] = b - ('a' - 'A')
		}
	}

	return string(out)
}

// lexUpdateExpression splits expr into words and delimiters. Placeholders
// (#name, :value) stay inside their word, so they can never be mistaken for a
// clause keyword.
func lexUpdateExpression(expr string) []exprToken {
	var tokens []exprToken

	for i := 0; i < len(expr); {
		switch {
		case isExpressionSpace(expr[i]):
			i++
		case isExpressionDelimiter(expr[i]):
			tokens = append(tokens, exprToken{text: expr[i : i+1], start: i, end: i + 1})
			i++
		default:
			start := i

			for i < len(expr) && !isExpressionSpace(expr[i]) && !isExpressionDelimiter(expr[i]) {
				i++
			}

			tokens = append(tokens, exprToken{text: expr[start:i], start: start, end: i, word: true})
		}
	}

	return tokens
}

// clauseKeyword returns the canonical clause keyword t spells, or "".
func clauseKeyword(t exprToken) string {
	if !t.word {
		return ""
	}

	switch kw := asciiUpper(t.text); kw {
	case updateActionSet, updateActionAdd, updateActionDel, updateActionRem:
		return kw
	default:
		return ""
	}
}

// parseUpdateClauses splits an update expression into its clauses and checks
// their syntax. Clauses are located on the raw expression, so attribute name
// placeholders that resolve to a keyword (#n = "Remove") are never mistaken for
// one; callers resolve names per clause afterwards.
//
// It returns a ValidationException TableError when a section is repeated, when
// the expression does not start with a clause keyword, or when a clause body is
// not valid.
func parseUpdateClauses(expr string) ([]updateClause, error) {
	tokens := lexUpdateExpression(expr)
	if len(tokens) == 0 {
		return nil, newValidationException(updateExpressionPrefix + "The expression can not be empty;")
	}

	p := &updateParser{expr: expr, tokens: tokens}

	var clauses []updateClause

	seen := make(map[string]struct{}, 4)

	for p.pos < len(tokens) {
		action := clauseKeyword(tokens[p.pos])
		if action == "" {
			return nil, p.syntaxError(p.pos)
		}

		if _, dup := seen[action]; dup {
			return nil, newValidationException(fmt.Sprintf(
				"%sThe %q section can only be used once in an update expression;", updateExpressionPrefix, action))
		}

		seen[action] = struct{}{}

		bodyStart := tokens[p.pos].end
		p.pos++

		p.end = p.clauseEnd()

		if err := p.parseBody(action); err != nil {
			return nil, err
		}

		bodyEnd := len(expr)
		if p.end < len(tokens) {
			bodyEnd = tokens[p.end].start
		}

		clauses = append(clauses, updateClause{action: action, body: strings.TrimSpace(expr[bodyStart:bodyEnd])})
		p.pos = p.end
	}

	return clauses, nil
}

// updateParser is a recursive-descent checker over the token stream. A clause
// body is the half-open token range [pos, end).
type updateParser struct {
	expr   string
	tokens []exprToken
	pos    int
	end    int
}

// clauseEnd returns the index of the next top-level clause keyword at or after
// pos, or len(tokens). Keywords inside parentheses belong to the body.
func (p *updateParser) clauseEnd() int {
	depth := 0

	for i := p.pos; i < len(p.tokens); i++ {
		switch t := p.tokens[i]; {
		case t.text == tokenOpenParen:
			depth++
		case t.text == tokenCloseP && depth > 0:
			depth--
		case depth == 0 && clauseKeyword(t) != "":
			return i
		}
	}

	return len(p.tokens)
}

// atEnd reports whether the current clause body is exhausted.
func (p *updateParser) atEnd() bool {
	return p.pos >= p.end
}

// peekIs reports whether the next token in the body is the delimiter text.
func (p *updateParser) peekIs(text string) bool {
	return !p.atEnd() && !p.tokens[p.pos].word && p.tokens[p.pos].text == text
}

// expect consumes the delimiter text or fails at the current token.
func (p *updateParser) expect(text string) error {
	if !p.peekIs(text) {
		return p.syntaxError(p.pos)
	}

	p.pos++

	return nil
}

// syntaxError builds the ValidationException for the unexpected token at index
// i; an index past the last token reports <EOF>.
func (p *updateParser) syntaxError(i int) error {
	if i >= len(p.tokens) {
		return newValidationException(fmt.Sprintf("%sSyntax error; token: %q, near: %q",
			updateExpressionPrefix, tokenEOF, p.tokens[len(p.tokens)-1].text))
	}

	from, to := i, i
	if i > 0 {
		from = i - 1
	}

	if i+1 < len(p.tokens) {
		to = i + 1
	}

	token, near := p.tokens[i].text, p.expr[p.tokens[from].start:p.tokens[to].end]

	return newValidationException(fmt.Sprintf("%sSyntax error; token: %q, near: %q", updateExpressionPrefix, token, near))
}

// parseBody checks the clause body against the grammar of action.
func (p *updateParser) parseBody(action string) error {
	for {
		if err := p.parseItem(action); err != nil {
			return err
		}

		if p.atEnd() {
			return nil
		}

		if err := p.expect(tokenComma); err != nil {
			return err
		}
	}
}

// parseItem checks one comma-separated action of a clause.
func (p *updateParser) parseItem(action string) error {
	if err := p.parseWord(); err != nil {
		return err
	}

	switch action {
	case updateActionSet:
		if err := p.expect(tokenAssign); err != nil {
			return err
		}

		return p.parseSetValue()
	case updateActionAdd, updateActionDel:
		return p.parseWord()
	default:
		return nil
	}
}

// parseWord consumes one path or placeholder.
func (p *updateParser) parseWord() error {
	if p.atEnd() || !p.tokens[p.pos].word {
		return p.syntaxError(p.pos)
	}

	p.pos++

	return nil
}

// parseSetValue checks "operand" or "operand (+|-) operand".
func (p *updateParser) parseSetValue() error {
	if err := p.parseOperand(); err != nil {
		return err
	}

	if p.peekIs(tokenPlus) || p.peekIs(tokenMinus) {
		p.pos++

		return p.parseOperand()
	}

	return nil
}

// parseOperand checks a path, a value placeholder or a function call such as
// if_not_exists(path, :v).
func (p *updateParser) parseOperand() error {
	if err := p.parseWord(); err != nil {
		return err
	}

	if !p.peekIs(tokenOpenParen) {
		return nil
	}

	p.pos++

	for {
		if err := p.parseOperand(); err != nil {
			return err
		}

		if !p.peekIs(tokenComma) {
			break
		}

		p.pos++
	}

	return p.expect(tokenCloseP)
}

// resolveUpdateNames replaces #name placeholders in body with their attribute
// names. Placeholders are matched as whole tokens, so #1 never rewrites the
// prefix of #10.
func resolveUpdateNames(body string, names map[string]string) string {
	if len(names) == 0 || !strings.Contains(body, "#") {
		return body
	}

	var sb strings.Builder

	for i := 0; i < len(body); {
		if body[i] != '#' {
			sb.WriteByte(body[i])

			i++

			continue
		}

		j := i + 1
		for j < len(body) && isPlaceholderChar(body[j]) {
			j++
		}

		if name, ok := names[body[i:j]]; ok {
			sb.WriteString(name)
		} else {
			sb.WriteString(body[i:j])
		}

		i = j
	}

	return sb.String()
}

func isPlaceholderChar(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
