package dynamodb

import "strings"

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

	names := projectionAttributeNames(projectionExpression, exprNames)
	projected := make(Item, len(names))
	seen := make(map[string]struct{}, len(names))

	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}

		if value, ok := item[name]; ok {
			projected[name] = value
		}
	}

	return projected
}

func projectionAttributeNames(projectionExpression string, exprNames map[string]string) []string {
	tokens := strings.Split(projectionExpression, ",")
	names := make([]string, 0, len(tokens))

	for _, token := range tokens {
		name := resolveProjectionToken(strings.TrimSpace(token), exprNames)
		if name != "" {
			names = append(names, name)
		}
	}

	return names
}

func resolveProjectionToken(token string, exprNames map[string]string) string {
	if token == "" {
		return ""
	}

	if name, ok := exprNames[token]; ok {
		return strings.TrimSpace(name)
	}

	resolved := token
	for placeholder, name := range exprNames {
		resolved = strings.ReplaceAll(resolved, placeholder, name)
	}

	return strings.TrimSpace(resolved)
}
