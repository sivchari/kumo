package cloudfront

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"
)

// CreateFunction stores a new function in the DEVELOPMENT stage. AWS
// rejects duplicate names; kumo mirrors that with a "duplicate name"
// error tagged for handler translation.
func (m *MemoryStorage) CreateFunction(_ context.Context, name, runtime, comment string, code []byte) (*Function, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Functions[name]; exists {
		return nil, &Error{Code: "FunctionAlreadyExists", Message: "Function " + name + " already exists"}
	}

	now := time.Now().UTC()

	fn := &Function{
		Name:             name,
		Runtime:          defaultRuntime(runtime),
		Comment:          comment,
		CreatedTime:      now,
		LastModifiedTime: now,
		Code:             map[string][]byte{FunctionStageDevelopment: append([]byte(nil), code...)},
		ETag:             functionETag(name, FunctionStageDevelopment, code, now),
	}

	m.Functions[name] = fn

	return fn, nil
}

// GetFunction returns the function metadata together with the raw code
// for the requested stage. AWS GetFunction (no /describe suffix)
// streams the code directly; the metadata comes back via headers.
func (m *MemoryStorage) GetFunction(_ context.Context, name, stage string) (*Function, []byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fn, ok := m.Functions[name]
	if !ok {
		return nil, nil, &Error{Code: "NoSuchFunctionExists", Message: "Function " + name + " does not exist"}
	}

	stage = effectiveStage(stage)

	code, ok := fn.Code[stage]
	if !ok {
		return nil, nil, &Error{Code: "NoSuchFunctionExists", Message: "Function " + name + " has no " + stage + " stage"}
	}

	return fn, append([]byte(nil), code...), nil
}

// DescribeFunction returns the metadata only (no code). AWS routes this
// through GET /function/{name}/describe.
func (m *MemoryStorage) DescribeFunction(ctx context.Context, name, stage string) (*Function, error) {
	fn, _, err := m.GetFunction(ctx, name, stage)
	if err != nil {
		return nil, err
	}

	return fn, nil
}

// ListFunctions returns every function that has the requested stage.
// Empty stage defaults to DEVELOPMENT (mirroring AWS).
func (m *MemoryStorage) ListFunctions(_ context.Context, stage string) ([]*Function, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stage = effectiveStage(stage)

	out := make([]*Function, 0, len(m.Functions))

	for _, fn := range m.Functions {
		if _, hasStage := fn.Code[stage]; !hasStage {
			continue
		}

		out = append(out, fn)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out, nil
}

// UpdateFunction overwrites the DEVELOPMENT stage. AWS requires the
// caller's If-Match to equal the current ETag — kumo enforces the same
// so optimistic-concurrency clients work.
func (m *MemoryStorage) UpdateFunction(_ context.Context, name, runtime, comment string, code []byte, ifMatch string) (*Function, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fn, ok := m.Functions[name]
	if !ok {
		return nil, &Error{Code: "NoSuchFunctionExists", Message: "Function " + name + " does not exist"}
	}

	if err := checkIfMatch(ifMatch, fn.ETag); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	fn.Runtime = defaultRuntime(runtime)
	fn.Comment = comment
	fn.LastModifiedTime = now

	fn.Code[FunctionStageDevelopment] = append([]byte(nil), code...)
	fn.ETag = functionETag(name, FunctionStageDevelopment, code, now)

	return fn, nil
}

// PublishFunction promotes the DEVELOPMENT code to LIVE. AWS allows
// re-publish (each call replaces LIVE); kumo follows.
func (m *MemoryStorage) PublishFunction(_ context.Context, name, ifMatch string) (*Function, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fn, ok := m.Functions[name]
	if !ok {
		return nil, &Error{Code: "NoSuchFunctionExists", Message: "Function " + name + " does not exist"}
	}

	if err := checkIfMatch(ifMatch, fn.ETag); err != nil {
		return nil, err
	}

	devCode, ok := fn.Code[FunctionStageDevelopment]
	if !ok {
		return nil, &Error{Code: "NoSuchFunctionExists", Message: "Function " + name + " has no DEVELOPMENT stage to publish"}
	}

	now := time.Now().UTC()

	fn.LastModifiedTime = now

	fn.Code[FunctionStageLive] = append([]byte(nil), devCode...)
	fn.ETag = functionETag(name, FunctionStageLive, devCode, now)

	return fn, nil
}

// DeleteFunction removes the function entirely. AWS requires
// disassociation from all distributions first; kumo doesn't model
// associations yet, so deletes always succeed when If-Match passes.
func (m *MemoryStorage) DeleteFunction(_ context.Context, name, ifMatch string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fn, ok := m.Functions[name]
	if !ok {
		return &Error{Code: "NoSuchFunctionExists", Message: "Function " + name + " does not exist"}
	}

	if err := checkIfMatch(ifMatch, fn.ETag); err != nil {
		return err
	}

	delete(m.Functions, name)

	return nil
}

// effectiveStage normalises the stage query parameter — AWS treats
// missing as DEVELOPMENT.
func effectiveStage(stage string) string {
	if stage == FunctionStageLive {
		return FunctionStageLive
	}

	return FunctionStageDevelopment
}

// defaultRuntime falls back to cloudfront-js-2.0 when the request
// omits Runtime; that's also what the AWS CLI defaults to today.
func defaultRuntime(runtime string) string {
	if runtime == "" {
		return "cloudfront-js-2.0"
	}

	return runtime
}

// functionETag is a deterministic SHA-256 of the inputs that change on
// every mutation. Real CloudFront uses an opaque string; the only
// requirement is uniqueness per version.
func functionETag(name, stage string, code []byte, now time.Time) string {
	h := sha256.New()
	_, _ = h.Write([]byte(name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(stage))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(code)
	_, _ = h.Write([]byte(now.Format(time.RFC3339Nano)))

	return "E" + hex.EncodeToString(h.Sum(nil))[:14]
}

// checkIfMatch returns an InvalidIfMatchVersion error when the caller
// supplied a stale or missing version.
func checkIfMatch(supplied, current string) error {
	if supplied == "" {
		return &Error{Code: "InvalidIfMatchVersion", Message: "The If-Match version is missing or invalid"}
	}

	if supplied != current {
		return &Error{Code: "PreconditionFailed", Message: "The precondition in the If-Match header failed"}
	}

	return nil
}
