package cligen

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"path"
	"sort"
	"text/template"

	"github.com/sivchari/kumo/internal/service"
)

//go:embed templates/service.go.tmpl
var templateFS embed.FS

var serviceTemplate = template.Must(template.ParseFS(templateFS, "templates/service.go.tmpl"))

// serviceView is the template data for one generated cli/gen_{service}.go file.
type serviceView struct {
	KumoName string
	// SDKIdent is the aws-sdk-go-v2 package's Go identifier (the last
	// import-path segment, e.g. "eventbridge"), used for {{.SDKIdent}}.Client
	// references. It can differ from KumoName: the kumo "events" service, for
	// example, wraps the aws-sdk-go-v2 "eventbridge" package.
	SDKIdent     string
	CLIName      string
	Use          string
	Short        string
	SDKImport    string
	HasOverrides bool
	StdImports   []string
	ThirdParty   []string
	Actions      []actionView
}

// actionView is the template data for one generated leaf command.
type actionView struct {
	FuncName       string
	Use            string
	Short          string
	SDKMethod      string
	InputTypeName  string
	OutputTypeName string
	HasOutput      bool
	Flags          []flagView
}

// flagView is the template data for one generated cobra flag, with all
// Go-source snippets precomputed so the template itself stays declarative.
type flagView struct {
	VarName  string
	VarDecl  string
	Register string
	// Assign is the Go source (one or more statements) that reads the flag
	// variable and, if set, assigns it onto `input`.
	Assign string
}

// Render discovers CLI-able actions from services and renders the generated
// CLI command files. It is pure: it never touches the filesystem. The
// returned map keys are relative file paths (e.g. "cli/gen_sqs.go"); values
// are gofmt-formatted Go source. Diagnostics explain every service/action
// cli-gen chose not to generate (or did generate).
func Render(services []service.Service) (map[string]string, []Diagnostic, error) {
	disc := Discover(services)

	byService := make(map[string][]Action)
	for _, a := range disc.Actions {
		byService[a.ServiceName] = append(byService[a.ServiceName], a)
	}

	names := make([]string, 0, len(byService))
	for name := range byService {
		names = append(names, name)
	}

	sort.Strings(names)

	files := make(map[string]string, len(names))

	diagnostics := append([]Diagnostic(nil), disc.Diagnostics...)

	for _, name := range names {
		content, svcDiags, err := renderService(name, byService[name])
		if err != nil {
			return nil, nil, fmt.Errorf("render service %q: %w", name, err)
		}

		diagnostics = append(diagnostics, svcDiags...)

		if content != "" {
			files["cli/gen_"+name+".go"] = content
		}
	}

	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Service != diagnostics[j].Service {
			return diagnostics[i].Service < diagnostics[j].Service
		}

		return diagnostics[i].Action < diagnostics[j].Action
	})

	return files, diagnostics, nil
}

func renderService(name string, actions []Action) (string, []Diagnostic, error) {
	binding := sdkBindings[name]

	sort.Slice(actions, func(i, j int) bool { return actions[i].SDKMethod < actions[j].SDKMethod })

	imports := newImportSet(binding.ImportPath)

	actionViews, diagnostics, err := renderActions(name, binding, actions, imports)
	if err != nil {
		return "", nil, err
	}

	if len(actionViews) == 0 {
		return "", diagnostics, nil
	}

	use := binding.CommandUse
	if use == "" {
		use = name
	}

	view := serviceView{
		KumoName:     name,
		SDKIdent:     path.Base(binding.ImportPath),
		CLIName:      binding.CLIName,
		Use:          use,
		Short:        binding.CLIName + " commands",
		SDKImport:    binding.ImportPath,
		HasOverrides: overrideServices[name],
		Actions:      actionViews,
	}
	view.StdImports, view.ThirdParty = imports.sorted()

	content, err := executeTemplate(&view)
	if err != nil {
		return "", nil, err
	}

	return content, diagnostics, nil
}

// renderActions builds an actionView (and a diagnostic) for every action
// cli-gen can auto-derive flags for, skipping (with a diagnostic) any action
// whose Input type requires a hand-written override.
func renderActions(name string, binding sdkBinding, actions []Action, imports *importSet) ([]actionView, []Diagnostic, error) {
	var (
		views       []actionView
		diagnostics []Diagnostic
	)

	for i := range actions {
		action := &actions[i]

		fields := DeriveFields(name, action.SDKMethod, action.InputType)
		if fields.RequiresOverride {
			diagnostics = append(diagnostics, Diagnostic{
				Service: name,
				Action:  action.SDKMethod,
				Level:   LevelSkip,
				Reason:  fields.OverrideReason,
			})

			continue
		}

		av, err := buildActionView(binding, action, fields.Flags, imports)
		if err != nil {
			return nil, nil, fmt.Errorf("action %s: %w", action.SDKMethod, err)
		}

		views = append(views, av)

		diagnostics = append(diagnostics, Diagnostic{
			Service: name,
			Action:  action.SDKMethod,
			Level:   LevelGenerated,
			Reason:  fmt.Sprintf("%d flag(s) derived", len(fields.Flags)),
		})
	}

	return views, diagnostics, nil
}

func executeTemplate(view *serviceView) (string, error) {
	var buf bytes.Buffer
	if err := serviceTemplate.Execute(&buf, view); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("gofmt generated source: %w", err)
	}

	return string(formatted), nil
}

func buildActionView(binding sdkBinding, action *Action, fields []FieldFlag, imports *importSet) (actionView, error) {
	use := toKebabCase(action.SDKMethod)

	av := actionView{
		FuncName:       "new" + binding.CLIName + action.SDKMethod + "Cmd",
		Use:            use,
		Short:          action.SDKMethod,
		SDKMethod:      action.SDKMethod,
		InputTypeName:  action.InputType.Name(),
		OutputTypeName: action.OutputType.Name(),
		HasOutput:      hasOutputContent(action.OutputType) && !isOutputPrintSuppressed(action.ServiceName, action.SDKMethod),
	}

	if av.HasOutput {
		imports.needJSONEncode()
	}

	for _, f := range fields {
		fv, err := buildFlagView(f, use, imports)
		if err != nil {
			return actionView{}, err
		}

		av.Flags = append(av.Flags, fv)
	}

	return av, nil
}
