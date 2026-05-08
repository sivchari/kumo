package cloudcontrol

import (
	"context"
	"sync"
)

// Handler implements Cloud Control CRUD for one CloudFormation-style
// resource type (e.g. "AWS::S3::Bucket"). Each method takes and returns
// the JSON-serialised resource state — Cloud Control's wire format is
// "DesiredState as a JSON string", so handlers operate on string-typed
// JSON to avoid double encode/decode.
type Handler interface {
	// TypeName returns the resource type the handler serves, e.g.
	// "AWS::S3::Bucket".
	TypeName() string

	// Create provisions a new resource from the supplied desired-state
	// JSON. Returns the primary identifier (the value Cloud Control uses
	// to address the resource on subsequent calls) and the read-back
	// state, which may differ from desired (server-assigned fields, etc.).
	Create(ctx context.Context, desiredState []byte) (identifier string, state []byte, err error)

	// Read returns the current state of the resource addressed by
	// identifier, or NotFoundError when it doesn't exist.
	Read(ctx context.Context, identifier string) (state []byte, err error)

	// Update applies a JSON Patch (RFC 6902) document to the existing
	// resource and returns the updated state. Patch documents are how
	// Cloud Control conveys updates on the wire.
	Update(ctx context.Context, identifier string, patchDocument []byte) (state []byte, err error)

	// Delete removes the resource. Returning NotFoundError is acceptable —
	// Cloud Control surfaces it as the resource already being absent.
	Delete(ctx context.Context, identifier string) error

	// List returns identifiers + state for every resource of this type.
	// kumo doesn't paginate Cloud Control responses today; pagination can
	// be added later through a separate List(ctx, after) signature without
	// breaking existing handlers.
	List(ctx context.Context) ([]ResourceDescription, error)
}

// ResourceDescription pairs an identifier with its serialised state, the
// shape Cloud Control returns from List and Get.
type ResourceDescription struct {
	Identifier string
	Properties []byte
}

// Registry maps a resource type name to its Handler.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register installs a handler for its declared TypeName. A previously
// registered handler with the same TypeName is replaced — this matters
// for tests that swap in fakes.
func (r *Registry) Register(h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[h.TypeName()] = h
}

// Get returns the handler for typeName, or nil + false when no handler is
// registered. Callers translate the false case into Cloud Control's
// "TypeNotFoundException".
func (r *Registry) Get(typeName string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.handlers[typeName]

	return h, ok
}

// defaultHandlers collects the handlers registered via init() in
// per-resource files. It is appended to from those init() functions so
// every imported handler shows up in defaultRegistry().
var defaultHandlers []Handler

// registerDefaultHandler appends a handler to the package-level list. It
// is the entry point per-resource init() functions use, e.g.:
//
//	func init() { registerDefaultHandler(&S3Bucket{}) }
//
// The handler will be present in every Service constructed via
// defaultRegistry() / init().
func registerDefaultHandler(h Handler) {
	defaultHandlers = append(defaultHandlers, h)
}

// defaultRegistry builds a Registry populated with everything registered
// via registerDefaultHandler. Tests that want a focused registry can call
// NewRegistry() and Register() directly instead.
func defaultRegistry() *Registry {
	r := NewRegistry()
	for _, h := range defaultHandlers {
		r.Register(h)
	}

	return r
}
