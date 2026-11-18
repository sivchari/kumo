package dynamodb

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

func projectItemsForExpression(items []Item, projectionExpression string, exprNames map[string]string) []Item {
	if strings.TrimSpace(projectionExpression) == "" {
		return items
	}

	projected := make([]Item, len(items))
	for i, item := range items {
		projected[i] = projectItemForExpression(item, projectionExpression, exprNames)
	}

	return projected
}

func projectItemForExpression(item Item, projectionExpression string, exprNames map[string]string) Item {
	if item == nil || strings.TrimSpace(projectionExpression) == "" {
		return item
	}

	root := &projectionNode{}

	for _, token := range strings.Split(projectionExpression, ",") {
		path := parseProjectionPath(strings.TrimSpace(token), exprNames)
		if len(path) == 0 || path[0].index >= 0 {
			continue
		}

		value, ok := item[path[0].name]
		// Paths missing from the item are skipped, as DynamoDB does.
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

// projectionSegment is one step of a document path: a map key, or a list
// index when index >= 0.
type projectionSegment struct {
	name  string
	index int
}

// parseProjectionPath splits a document path such as "#m.a[1].b" into
// segments, resolving expression attribute names per segment.
func parseProjectionPath(token string, exprNames map[string]string) []projectionSegment {
	if token == "" {
		return nil
	}

	var path []projectionSegment

	for _, part := range strings.Split(token, ".") {
		name, rest, _ := strings.Cut(part, "[")
		name = strings.TrimSpace(name)

		if resolved, ok := exprNames[name]; ok {
			name = resolved
		}

		if name == "" {
			return nil
		}

		path = append(path, projectionSegment{name: name, index: -1})

		for rest != "" {
			idx, after, ok := strings.Cut(rest, "]")
			if !ok {
				return nil
			}

			n, err := strconv.Atoi(strings.TrimSpace(idx))
			if err != nil || n < 0 {
				return nil
			}

			path = append(path, projectionSegment{index: n})
			rest = strings.TrimPrefix(strings.TrimSpace(after), "[")
		}
	}

	return path
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
