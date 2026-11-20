package dynamodb

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// errMalformedProjectionPath marks a document path that cannot be parsed; the
// path is skipped rather than rejected.
var errMalformedProjectionPath = errors.New("dynamodb: malformed projection path")

// projection is a parsed ProjectionExpression: the document paths to keep.
// The zero value keeps every attribute.
type projection struct {
	active bool
	paths  [][]projectionSegment
}

// parseProjectionExpression parses and validates a ProjectionExpression,
// returning a ValidationException for an undefined expression attribute name
// or for document paths that overlap. An empty expression yields a projection
// that keeps every attribute.
func parseProjectionExpression(expression string, exprNames map[string]string) (projection, error) {
	if strings.TrimSpace(expression) == "" {
		return projection{}, nil
	}

	var paths [][]projectionSegment

	for _, token := range strings.Split(expression, ",") {
		path, err := parseProjectionPath(strings.TrimSpace(token), exprNames)
		if errors.Is(err, errMalformedProjectionPath) {
			continue
		}

		if err != nil {
			return projection{}, err
		}

		paths = append(paths, path)
	}

	for i, one := range paths {
		for _, two := range paths[i+1:] {
			if pathsOverlap(one, two) {
				return projection{}, newValidationException(fmt.Sprintf(
					"Invalid ProjectionExpression: Two document paths overlap with each other; "+
						"must remove or rewrite one of these paths; path one: %s, path two: %s",
					formatProjectionPath(one), formatProjectionPath(two)))
			}
		}
	}

	return projection{active: true, paths: paths}, nil
}

// projectAll applies the projection to every item.
func (p projection) projectAll(items []Item) []Item {
	if !p.active {
		return items
	}

	projected := make([]Item, len(items))
	for i, item := range items {
		projected[i] = p.project(item)
	}

	return projected
}

// project returns the parts of item selected by the projection. Paths that do
// not exist in the item are skipped, as DynamoDB does.
func (p projection) project(item Item) Item {
	if !p.active || item == nil {
		return item
	}

	root := &projectionNode{}

	for _, path := range p.paths {
		if path[0].index >= 0 {
			continue
		}

		value, ok := item[path[0].name]
		if !ok || !pathExists(&value, path[1:]) {
			continue
		}

		root.child(path[0]).add(&value, path[1:])
	}

	projected := make(Item, len(root.fields))
	for name, node := range root.fields {
		projected[name] = node.build()
	}

	return projected
}

// pathsOverlap reports whether one path is equal to, or a prefix of, the other.
func pathsOverlap(a, b []projectionSegment) bool {
	n := min(len(a), len(b))

	return slices.Equal(a[:n], b[:n])
}

// formatProjectionPath renders a path as DynamoDB reports it in validation
// messages, e.g. "[a, [0], b]".
func formatProjectionPath(path []projectionSegment) string {
	parts := make([]string, len(path))

	for i, seg := range path {
		if seg.index >= 0 {
			parts[i] = "[" + strconv.Itoa(seg.index) + "]"
		} else {
			parts[i] = seg.name
		}
	}

	return "[" + strings.Join(parts, ", ") + "]"
}

// projectionSegment is one step of a document path: a map key, or a list
// index when index >= 0.
type projectionSegment struct {
	name  string
	index int
}

// parseProjectionPath splits a document path such as "#m.a[1].b" into
// segments, resolving expression attribute names per segment. It returns
// errMalformedProjectionPath for an unparseable path and a ValidationException
// for a "#name" placeholder missing from exprNames.
func parseProjectionPath(token string, exprNames map[string]string) ([]projectionSegment, error) {
	if token == "" {
		return nil, errMalformedProjectionPath
	}

	var path []projectionSegment

	for _, part := range strings.Split(token, ".") {
		name, rest, _ := strings.Cut(part, "[")
		name = strings.TrimSpace(name)

		if strings.HasPrefix(name, "#") {
			resolved, ok := exprNames[name]
			if !ok {
				return nil, newValidationException(
					"Invalid ProjectionExpression: An expression attribute name used in the document path is not defined; attribute name: " + name)
			}

			name = resolved
		}

		if name == "" {
			return nil, errMalformedProjectionPath
		}

		path = append(path, projectionSegment{name: name, index: -1})

		for rest != "" {
			idx, after, ok := strings.Cut(rest, "]")
			if !ok {
				return nil, errMalformedProjectionPath
			}

			n, err := strconv.Atoi(strings.TrimSpace(idx))
			if err != nil || n < 0 {
				return nil, errMalformedProjectionPath
			}

			path = append(path, projectionSegment{index: n})
			rest = strings.TrimPrefix(strings.TrimSpace(after), "[")
		}
	}

	return path, nil
}

// projectionNode collects the parts of one attribute selected by a
// projection, so that "m.a, m.b" and "l[0], l[2]" merge into one value.
type projectionNode struct {
	whole  *AttributeValue
	fields map[string]*projectionNode
	elems  map[int]*projectionNode
}

func (n *projectionNode) child(seg projectionSegment) *projectionNode {
	if seg.index >= 0 {
		if n.elems == nil {
			n.elems = make(map[int]*projectionNode)
		}

		if n.elems[seg.index] == nil {
			n.elems[seg.index] = &projectionNode{}
		}

		return n.elems[seg.index]
	}

	if n.fields == nil {
		n.fields = make(map[string]*projectionNode)
	}

	if n.fields[seg.name] == nil {
		n.fields[seg.name] = &projectionNode{}
	}

	return n.fields[seg.name]
}

// add records the part of value that path selects; the caller checks the
// path exists.
func (n *projectionNode) add(value *AttributeValue, path []projectionSegment) {
	for _, seg := range path {
		if seg.index >= 0 {
			value = value.L[seg.index]
		} else {
			value = value.M[seg.name]
		}

		n = n.child(seg)
	}

	n.whole = value
}

func pathExists(value *AttributeValue, path []projectionSegment) bool {
	for _, seg := range path {
		switch {
		case seg.index >= 0 && seg.index < len(value.L):
			value = value.L[seg.index]
		case seg.index < 0 && value.M != nil:
			value = value.M[seg.name]
		default:
			return false
		}

		if value == nil {
			return false
		}
	}

	return true
}

func (n *projectionNode) build() AttributeValue {
	if n.whole != nil {
		return *n.whole
	}

	if n.elems != nil {
		list := make([]*AttributeValue, 0, len(n.elems))

		for _, i := range slices.Sorted(maps.Keys(n.elems)) {
			v := n.elems[i].build()
			list = append(list, &v)
		}

		return AttributeValue{L: list}
	}

	m := make(map[string]*AttributeValue, len(n.fields))

	for name, child := range n.fields {
		v := child.build()
		m[name] = &v
	}

	return AttributeValue{M: m}
}
