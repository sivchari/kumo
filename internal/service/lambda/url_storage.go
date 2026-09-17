package lambda

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"slices"
	"time"
)

const (
	functionURLIDLength   = 32
	functionURLIDAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

	msgFunctionURLNotFound = "The resource you requested does not exist."
)

// CreateFunctionURLConfig creates the function URL of functionName. A second
// URL for the same function is a conflict, as on AWS.
func (s *MemoryStorage) CreateFunctionURLConfig(_ context.Context, functionName string, spec FunctionURLConfigSpec) (*FunctionURLConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn, err := s.functionLocked(functionName)
	if err != nil {
		return nil, err
	}

	if _, exists := s.FunctionURLs[functionName]; exists {
		return nil, &FunctionError{
			Type:    ErrResourceConflict,
			Message: fmt.Sprintf("Failed to create function url config for [functionArn = %s]. Error message:  FunctionUrlConfig exists for this Lambda function", fn.FunctionArn),
		}
	}

	urlID, err := s.uniqueFunctionURLIDLocked()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	cfg := &FunctionURLConfig{
		URLID:            urlID,
		FunctionArn:      fn.FunctionArn,
		FunctionURL:      fmt.Sprintf("https://%s.lambda-url.%s.on.aws/", urlID, s.region),
		AuthType:         spec.AuthType,
		InvokeMode:       spec.InvokeMode,
		Cors:             spec.Cors.clone(),
		CreationTime:     now,
		LastModifiedTime: now,
	}

	s.FunctionURLs[functionName] = cfg
	s.saveLocked()

	return cfg.clone(), nil
}

// GetFunctionURLConfig returns a copy of the function URL of functionName.
func (s *MemoryStorage) GetFunctionURLConfig(_ context.Context, functionName string) (*FunctionURLConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, err := s.functionURLLocked(functionName)
	if err != nil {
		return nil, err
	}

	return cfg.clone(), nil
}

// UpdateFunctionURLConfig applies the present fields of update to the function
// URL of functionName and returns a copy of the result.
func (s *MemoryStorage) UpdateFunctionURLConfig(_ context.Context, functionName string, update FunctionURLConfigUpdate) (*FunctionURLConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := s.functionURLLocked(functionName)
	if err != nil {
		return nil, err
	}

	if update.AuthType != nil {
		cfg.AuthType = *update.AuthType
	}

	if update.InvokeMode != nil {
		cfg.InvokeMode = *update.InvokeMode
	}

	switch {
	case update.ClearCors:
		cfg.Cors = nil
	case update.Cors != nil:
		cfg.Cors = update.Cors.clone()
	}

	cfg.LastModifiedTime = time.Now().UTC()

	s.saveLocked()

	return cfg.clone(), nil
}

// DeleteFunctionURLConfig removes the function URL of functionName.
func (s *MemoryStorage) DeleteFunctionURLConfig(_ context.Context, functionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.functionURLLocked(functionName); err != nil {
		return err
	}

	delete(s.FunctionURLs, functionName)
	s.saveLocked()

	return nil
}

// ListFunctionURLConfigs returns the function URLs of functionName: an empty,
// non-nil slice or the single URL a function can have.
func (s *MemoryStorage) ListFunctionURLConfigs(_ context.Context, functionName string) ([]*FunctionURLConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, err := s.functionLocked(functionName); err != nil {
		return nil, err
	}

	configs := make([]*FunctionURLConfig, 0, 1)
	if cfg, exists := s.FunctionURLs[functionName]; exists {
		configs = append(configs, cfg.clone())
	}

	return configs, nil
}

// functionLocked returns the function or the not-found error; the caller holds the lock.
func (s *MemoryStorage) functionLocked(functionName string) (*Function, error) {
	fn, exists := s.Functions[functionName]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", functionName),
		}
	}

	return fn, nil
}

// functionURLLocked returns the stored URL config of an existing function; the
// caller holds the lock.
func (s *MemoryStorage) functionURLLocked(functionName string) (*FunctionURLConfig, error) {
	if _, err := s.functionLocked(functionName); err != nil {
		return nil, err
	}

	cfg, exists := s.FunctionURLs[functionName]
	if !exists {
		return nil, &FunctionError{Type: ErrResourceNotFound, Message: msgFunctionURLNotFound}
	}

	return cfg, nil
}

// uniqueFunctionURLIDLocked draws url ids until one is not in use; the caller holds the lock.
func (s *MemoryStorage) uniqueFunctionURLIDLocked() (string, error) {
	for {
		id, err := newFunctionURLID()
		if err != nil {
			return "", err
		}

		if !s.functionURLIDInUseLocked(id) {
			return id, nil
		}
	}
}

func (s *MemoryStorage) functionURLIDInUseLocked(id string) bool {
	for _, cfg := range s.FunctionURLs {
		if cfg.URLID == id {
			return true
		}
	}

	return false
}

// newFunctionURLID returns a 32-character lowercase alphanumeric url id, the
// shape AWS uses in <url-id>.lambda-url.<region>.on.aws.
func newFunctionURLID() (string, error) {
	id := make([]byte, functionURLIDLength)
	alphabetSize := big.NewInt(int64(len(functionURLIDAlphabet)))

	for i := range id {
		n, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", fmt.Errorf("generate function url id: %w", err)
		}

		id[i] = functionURLIDAlphabet[n.Int64()]
	}

	return string(id), nil
}

func (c *FunctionURLConfig) clone() *FunctionURLConfig {
	if c == nil {
		return nil
	}

	out := *c
	out.Cors = c.Cors.clone()

	return &out
}

// isEmpty reports whether no CORS setting is present. The API sends an empty
// Cors object to remove the configuration, so an empty object is never stored.
func (c *FunctionURLCORS) isEmpty() bool {
	return c == nil || (!c.AllowCredentials && c.MaxAge == 0 &&
		len(c.AllowHeaders) == 0 && len(c.AllowMethods) == 0 && len(c.AllowOrigins) == 0 && len(c.ExposeHeaders) == 0)
}

func (c *FunctionURLCORS) clone() *FunctionURLCORS {
	if c == nil {
		return nil
	}

	out := *c
	out.AllowHeaders = slices.Clone(c.AllowHeaders)
	out.AllowMethods = slices.Clone(c.AllowMethods)
	out.AllowOrigins = slices.Clone(c.AllowOrigins)
	out.ExposeHeaders = slices.Clone(c.ExposeHeaders)

	return &out
}
