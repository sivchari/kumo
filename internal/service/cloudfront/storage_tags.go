package cloudfront

import (
	"context"
	"fmt"
	"maps"
)

// TagResource adds or overwrites tags on the distribution identified by arn.
func (s *MemoryStorage) TagResource(_ context.Context, arn string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dist, err := s.distributionByARNLocked(arn)
	if err != nil {
		return err
	}

	if dist.Tags == nil {
		dist.Tags = make(map[string]string, len(tags))
	}

	maps.Copy(dist.Tags, tags)

	s.saveLocked()

	return nil
}

// UntagResource removes the given tag keys from the distribution identified by arn.
func (s *MemoryStorage) UntagResource(_ context.Context, arn string, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dist, err := s.distributionByARNLocked(arn)
	if err != nil {
		return err
	}

	for _, k := range keys {
		delete(dist.Tags, k)
	}

	s.saveLocked()

	return nil
}

// ListTagsForResource returns a copy of the tags on the distribution identified by arn.
func (s *MemoryStorage) ListTagsForResource(_ context.Context, arn string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dist, err := s.distributionByARNLocked(arn)
	if err != nil {
		return nil, err
	}

	return maps.Clone(dist.Tags), nil
}

// distributionByARNLocked finds the distribution with the given ARN. The
// caller must hold s.mu.
func (s *MemoryStorage) distributionByARNLocked(arn string) (*Distribution, error) {
	for _, d := range s.Distributions {
		if d.ARN == arn {
			return d, nil
		}
	}

	return nil, &Error{
		Code:    errNoSuchResource,
		Message: fmt.Sprintf("The resource %s does not exist", arn),
	}
}
