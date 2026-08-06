package cligen

import (
	"context"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// actionMethodBlocklist lists exported methods on a JSONProtocolService that
// must never be treated as a discovered action, even though some happen to
// match the func(http.ResponseWriter, *http.Request) handler signature (most
// notably DispatchAction itself). Kept explicit and over-inclusive for
// defense in depth: a wrongly-blocked handler only means a missed action
// (visible as a stale-looking diagnostic count), while a wrongly-allowed
// lifecycle method would silently generate a broken command.
var actionMethodBlocklist = map[string]bool{
	"Name":               true,
	"RegisterRoutes":     true,
	"DispatchAction":     true,
	"TargetPrefix":       true,
	"JSONProtocol":       true,
	"Meta":               true,
	"Close":              true,
	"ServeHTTP":          true,
	"QueryProtocol":      true,
	"ServiceIdentifier":  true,
	"Actions":            true,
	"CBORProtocol":       true,
	"ServiceName":        true,
	"DispatchCBORAction": true,
	"HandleExecuteAPI":   true,
	"Storage":            true,
	"BaseURL":            true,
}

var (
	responseWriterType = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
	requestPtrType     = reflect.TypeOf(&http.Request{})
	contextType        = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType          = reflect.TypeOf((*error)(nil)).Elem()
)

// DiscoverResult is the outcome of walking service.Services(): the resolved,
// SDK-matched actions ready for flag derivation, and diagnostics for every
// service or action cli-gen chose not to generate.
type DiscoverResult struct {
	Actions     []Action
	Diagnostics []Diagnostic
}

// Discover walks services and resolves auto-discoverable actions against the
// aws-sdk-go-v2 client method sets registered in sdkBindings. It never
// panics or errors on an unrecognized service: anything it cannot resolve is
// recorded as a skip Diagnostic instead.
func Discover(services []service.Service) DiscoverResult {
	svcs := make([]service.Service, len(services))
	copy(svcs, services)
	sort.Slice(svcs, func(i, j int) bool { return svcs[i].Name() < svcs[j].Name() })

	var result DiscoverResult

	for _, svc := range svcs {
		result.discoverService(svc)
	}

	sort.Slice(result.Actions, func(i, j int) bool {
		if result.Actions[i].ServiceName != result.Actions[j].ServiceName {
			return result.Actions[i].ServiceName < result.Actions[j].ServiceName
		}

		return result.Actions[i].SDKMethod < result.Actions[j].SDKMethod
	})

	return result
}

func (r *DiscoverResult) discoverService(svc service.Service) {
	name := svc.Name()

	goNames, discoverable := protocolActionNames(svc)
	if !discoverable {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Service: name,
			Level:   LevelSkip,
			Reason:  "REST-only service (no Query/JSON protocol), skipping auto-discovery",
		})

		return
	}

	binding, ok := sdkBindings[name]
	if !ok {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Service: name,
			Level:   LevelSkip,
			Reason:  "no aws-sdk-go-v2 client binding registered for this service, skipping auto-discovery",
		})

		return
	}

	for _, goName := range goNames {
		r.discoverAction(name, binding, goName)
	}
}

func (r *DiscoverResult) discoverAction(serviceName string, binding sdkBinding, goName string) {
	m, matched := resolveSDKMethod(binding.ClientType, goName)
	if !matched {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Service: serviceName,
			Action:  goName,
			Level:   LevelSkip,
			Reason:  "no matching aws-sdk-go-v2 client method (likely a kumo-only custom endpoint)",
		})

		return
	}

	if isSkippedAction(serviceName, m.Name) {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Service: serviceName,
			Action:  m.Name,
			Level:   LevelSkip,
			Reason:  "manually overridden in cli/" + strings.ToLower(serviceName) + "_overrides.go, skipping auto-generation",
		})

		return
	}

	r.Actions = append(r.Actions, Action{
		ServiceName: serviceName,
		GoMethod:    goName,
		SDKMethod:   m.Name,
		InputType:   m.Type.In(2).Elem(),
		OutputType:  m.Type.Out(0).Elem(),
	})
}

// protocolActionNames returns the Go-facing action/method names discovered
// for svc without SDK resolution, plus whether svc was recognized as an
// auto-discoverable (Query or JSON protocol) service at all.
func protocolActionNames(svc service.Service) (names []string, discoverable bool) {
	if qp, ok := svc.(service.QueryProtocolService); ok {
		return append([]string(nil), qp.Actions()...), true
	}

	if _, ok := svc.(service.JSONProtocolService); ok {
		return jsonHandlerMethodNames(svc), true
	}

	return nil, false
}

// jsonHandlerMethodNames reflects over svc's method set and returns the
// exported methods matching the func(http.ResponseWriter, *http.Request)
// handler signature, excluding actionMethodBlocklist entries.
func jsonHandlerMethodNames(svc service.Service) []string {
	t := reflect.TypeOf(svc)

	var names []string

	for i := range t.NumMethod() {
		m := t.Method(i)
		if actionMethodBlocklist[m.Name] {
			continue
		}

		if !isHandlerSignature(m.Type) {
			continue
		}

		names = append(names, m.Name)
	}

	sort.Strings(names)

	return names
}

// isHandlerSignature reports whether t is a method type (including receiver)
// matching func(http.ResponseWriter, *http.Request).
func isHandlerSignature(t reflect.Type) bool {
	return t.NumIn() == 3 && t.NumOut() == 0 &&
		t.In(1) == responseWriterType &&
		t.In(2) == requestPtrType
}

// resolveSDKMethod finds the method on *clientType whose name matches name
// case-insensitively and whose signature matches an aws-sdk-go-v2 client
// operation: func(*Client, context.Context, *XxxInput, ...func(*Options)) (*XxxOutput, error).
func resolveSDKMethod(clientType reflect.Type, name string) (reflect.Method, bool) {
	ptr := reflect.PointerTo(clientType)

	for i := range ptr.NumMethod() {
		m := ptr.Method(i)
		if strings.EqualFold(m.Name, name) && isSDKOperationSignature(m.Type) {
			return m, true
		}
	}

	return reflect.Method{}, false
}

func isSDKOperationSignature(t reflect.Type) bool {
	if t.NumIn() != 4 || !t.IsVariadic() || t.NumOut() != 2 {
		return false
	}

	if t.In(1) != contextType {
		return false
	}

	if t.In(2).Kind() != reflect.Pointer || t.In(2).Elem().Kind() != reflect.Struct {
		return false
	}

	if t.Out(0).Kind() != reflect.Pointer || t.Out(0).Elem().Kind() != reflect.Struct {
		return false
	}

	return t.Out(1) == errorType
}
