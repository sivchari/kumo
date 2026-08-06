package cligen

import (
	"fmt"
	"go/token"
	"strings"
)

// reservedLocalNames are identifiers already used inside a generated leaf
// command's RunE closure (see templates/service.go.tmpl), plus predeclared
// Go identifiers that a field name could collide with (e.g. sfn's
// SendTaskFailure.Error field would otherwise lower-case to "error",
// shadowing the builtin error type that RunE's own signature needs). A flag
// variable name colliding with one of these is renamed by appending "Value".
var reservedLocalNames = map[string]bool{
	"cmd": true, "err": true, "cfg": true, "client": true,
	"input": true, "out": true, "ctx": true, "decoded": true,
	"parsed": true, "val": true, "raw": true,
	"error": true,
}

// buildFlagView computes the Go source snippets (variable declaration, flag
// registration, and input-field assignment) for one derived flag, and
// records any imports its FlagKind requires on imports.
func buildFlagView(f *FieldFlag, actionUse string, imports *importSet) (flagView, error) {
	v := flagVarName(f.FieldName)
	desc := toWords(f.FieldName)

	switch f.Kind {
	case FlagString:
		return stringFlag(f, v, desc), nil
	case FlagInt32:
		return intFlag(f, v, desc, "Int32Var", "aws.Int32"), nil
	case FlagInt64:
		return intFlag(f, v, desc, "Int64Var", "aws.Int64"), nil
	case FlagBool:
		return boolFlag(f, v, desc), nil
	case FlagStringEnum:
		imports.needTypes()

		return stringEnumFlag(f, v, desc), nil
	case FlagBytesBase64:
		imports.needBase64()

		return bytesBase64Flag(f, v, desc, actionUse), nil
	case FlagTime:
		imports.needTime()

		return timeFlag(f, v, desc, actionUse), nil
	case FlagStringArray:
		return stringArrayFlag(f, v, desc), nil
	case FlagStringEnumArray:
		imports.needTypes()

		return stringEnumArrayFlag(f, v, desc), nil
	case FlagStringMap, FlagJSONBlob:
		imports.needJSON()

		return jsonBlobFlag(f, v, desc, actionUse), nil
	case FlagKV:
		imports.needKV()
		imports.needTypes()

		return kvFlag(f, v, desc, actionUse), nil
	case FlagKVArray:
		imports.needKV()
		imports.needTypes()

		return kvArrayFlag(f, v, desc, actionUse), nil
	case FlagCustomFunc:
		return customFuncFlag(f, v, desc, actionUse), nil
	default:
		return flagView{}, fmt.Errorf("unsupported flag kind %v for field %s", f.Kind, f.FieldName)
	}
}

// flagVarName derives a local RunE variable name from a struct field name,
// avoiding collisions with reservedLocalNames and Go keywords (e.g. a field
// named "Type" would otherwise lower-case to the reserved word "type").
func flagVarName(fieldName string) string {
	if fieldName == "" {
		return "value"
	}

	v := strings.ToLower(fieldName[:1]) + fieldName[1:]
	if reservedLocalNames[v] || token.IsKeyword(v) {
		v += "Value"
	}

	return v
}

func stringFlag(f *FieldFlag, v, desc string) flagView {
	assign := fmt.Sprintf("if %s != \"\" {\n\tinput.%s = %s\n}", v, f.FieldName, pointerWrap(f, "aws.String", v))
	if !f.Pointer {
		assign = fmt.Sprintf("if %s != \"\" {\n\tinput.%s = %s\n}", v, f.FieldName, v)
	}

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s string", v),
		Register: fmt.Sprintf("cmd.Flags().StringVar(&%s, %q, \"\", %q)", v, f.FlagName, desc),
		Assign:   assign,
	}
}

func intFlag(f *FieldFlag, v, desc, registerFn, wrapFn string) flagView {
	goType := "int32"
	if registerFn == "Int64Var" {
		goType = "int64"
	}

	assign := fmt.Sprintf("if %s != 0 {\n\tinput.%s = %s\n}", v, f.FieldName, pointerWrap(f, wrapFn, v))
	if !f.Pointer {
		assign = fmt.Sprintf("if %s != 0 {\n\tinput.%s = %s\n}", v, f.FieldName, v)
	}

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s %s", v, goType),
		Register: fmt.Sprintf("cmd.Flags().%s(&%s, %q, 0, %q)", registerFn, v, f.FlagName, desc),
		Assign:   assign,
	}
}

func boolFlag(f *FieldFlag, v, desc string) flagView {
	assign := fmt.Sprintf("if %s {\n\tinput.%s = %s\n}", v, f.FieldName, pointerWrap(f, "aws.Bool", v))
	if !f.Pointer {
		assign = fmt.Sprintf("if %s {\n\tinput.%s = %s\n}", v, f.FieldName, v)
	}

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s bool", v),
		Register: fmt.Sprintf("cmd.Flags().BoolVar(&%s, %q, false, %q)", v, f.FlagName, desc),
		Assign:   assign,
	}
}

func stringEnumFlag(f *FieldFlag, v, desc string) flagView {
	typeName := "types." + f.ElemType.Name()

	assign := fmt.Sprintf("if %s != \"\" {\n\tinput.%s = %s(%s)\n}", v, f.FieldName, typeName, v)
	if f.Pointer {
		assign = fmt.Sprintf("if %s != \"\" {\n\tval := %s(%s)\n\tinput.%s = &val\n}", v, typeName, v, f.FieldName)
	}

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s string", v),
		Register: fmt.Sprintf("cmd.Flags().StringVar(&%s, %q, \"\", %q)", v, f.FlagName, desc),
		Assign:   assign,
	}
}

func bytesBase64Flag(f *FieldFlag, v, desc, actionUse string) flagView {
	assign := fmt.Sprintf(
		"if %s != \"\" {\n\tdecoded, err := base64.StdEncoding.DecodeString(%s)\n\tif err != nil {\n\t\treturn fmt.Errorf(\"%s: decode --%s: %%w\", err)\n\t}\n\tinput.%s = decoded\n}",
		v, v, actionUse, f.FlagName, f.FieldName,
	)

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s string", v),
		Register: fmt.Sprintf("cmd.Flags().StringVar(&%s, %q, \"\", %q)", v, f.FlagName, desc+" (base64-encoded)"),
		Assign:   assign,
	}
}

func timeFlag(f *FieldFlag, v, desc, actionUse string) flagView {
	set := fmt.Sprintf("input.%s = parsed", f.FieldName)
	if f.Pointer {
		set = fmt.Sprintf("input.%s = &parsed", f.FieldName)
	}

	assign := fmt.Sprintf(
		"if %s != \"\" {\n\tparsed, err := time.Parse(time.RFC3339, %s)\n\tif err != nil {\n\t\treturn fmt.Errorf(\"%s: parse --%s: %%w\", err)\n\t}\n\t%s\n}",
		v, v, actionUse, f.FlagName, set,
	)

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s string", v),
		Register: fmt.Sprintf("cmd.Flags().StringVar(&%s, %q, \"\", %q)", v, f.FlagName, desc+" (RFC3339)"),
		Assign:   assign,
	}
}

func stringArrayFlag(f *FieldFlag, v, desc string) flagView {
	assign := fmt.Sprintf("if len(%s) > 0 {\n\tinput.%s = %s\n}", v, f.FieldName, v)

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s []string", v),
		Register: fmt.Sprintf("cmd.Flags().StringArrayVar(&%s, %q, nil, %q)", v, f.FlagName, desc),
		Assign:   assign,
	}
}

func stringEnumArrayFlag(f *FieldFlag, v, desc string) flagView {
	typeName := "types." + f.ElemType.Name()

	assign := fmt.Sprintf(
		"for _, s := range %s {\n\tinput.%s = append(input.%s, %s(s))\n}",
		v, f.FieldName, f.FieldName, typeName,
	)

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s []string", v),
		Register: fmt.Sprintf("cmd.Flags().StringArrayVar(&%s, %q, nil, %q)", v, f.FlagName, desc),
		Assign:   assign,
	}
}

func jsonBlobFlag(f *FieldFlag, v, desc, actionUse string) flagView {
	assign := fmt.Sprintf(
		"if %s != \"\" {\n\tif err := json.Unmarshal([]byte(%s), &input.%s); err != nil {\n\t\treturn fmt.Errorf(\"%s: parse --%s: %%w\", err)\n\t}\n}",
		v, v, f.FieldName, actionUse, f.FlagName,
	)

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s string", v),
		Register: fmt.Sprintf("cmd.Flags().StringVar(&%s, %q, \"\", %q)", v, f.FlagName, desc+" (JSON)"),
		Assign:   assign,
	}
}

func kvFlag(f *FieldFlag, v, desc, actionUse string) flagView {
	typeName := "types." + f.ElemType.Name()

	set := fmt.Sprintf("input.%s = val", f.FieldName)
	if f.Pointer {
		set = fmt.Sprintf("input.%s = &val", f.FieldName)
	}

	assign := fmt.Sprintf(
		"if %s != \"\" {\n\tvar val %s\n\tif err := cligen.SetFieldsFromKV(reflect.ValueOf(&val).Elem(), %s); err != nil {\n\t\treturn fmt.Errorf(\"%s: parse --%s: %%w\", err)\n\t}\n\t%s\n}",
		v, typeName, v, actionUse, f.FlagName, set,
	)

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s string", v),
		Register: fmt.Sprintf("cmd.Flags().StringVar(&%s, %q, \"\", %q)", v, f.FlagName, desc+" (key=value,key2=value2)"),
		Assign:   assign,
	}
}

func kvArrayFlag(f *FieldFlag, v, desc, actionUse string) flagView {
	typeName := "types." + f.ElemType.Name()

	assign := fmt.Sprintf(
		"for _, raw := range %s {\n\tvar val %s\n\tif err := cligen.SetFieldsFromKV(reflect.ValueOf(&val).Elem(), raw); err != nil {\n\t\treturn fmt.Errorf(\"%s: parse --%s: %%w\", err)\n\t}\n\tinput.%s = append(input.%s, val)\n}",
		v, typeName, actionUse, f.FlagName, f.FieldName, f.FieldName,
	)

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s []string", v),
		Register: fmt.Sprintf("cmd.Flags().StringArrayVar(&%s, %q, nil, %q)", v, f.FlagName, desc+" (key=value,key2=value2, repeatable)"),
		Assign:   assign,
	}
}

// customFuncFlag builds a flag for a FlagCustomFunc field: a raw string flag
// that, when non-empty, is decoded by calling f.CustomFunc (a hand-written
// cli package function, e.g. decodeDynamoDBJSON) instead of json.Unmarshal.
func customFuncFlag(f *FieldFlag, v, desc, actionUse string) flagView {
	set := fmt.Sprintf("input.%s = decoded", f.FieldName)
	if f.Pointer {
		set = fmt.Sprintf("input.%s = &decoded", f.FieldName)
	}

	assign := fmt.Sprintf(
		"if %s != \"\" {\n\tdecoded, err := %s(%s)\n\tif err != nil {\n\t\treturn fmt.Errorf(\"%s: parse --%s: %%w\", err)\n\t}\n\t%s\n}",
		v, f.CustomFunc, v, actionUse, f.FlagName, set,
	)

	return flagView{
		VarName:  v,
		VarDecl:  fmt.Sprintf("var %s string", v),
		Register: fmt.Sprintf("cmd.Flags().StringVar(&%s, %q, \"\", %q)", v, f.FlagName, desc+" (JSON)"),
		Assign:   assign,
	}
}

// pointerWrap returns wrapFn(v) when f.Pointer is true; used by scalar flag
// builders whose Input field is *T rather than T.
func pointerWrap(f *FieldFlag, wrapFn, v string) string {
	if !f.Pointer {
		return v
	}

	return fmt.Sprintf("%s(%s)", wrapFn, v)
}
