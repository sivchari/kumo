package sfn

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// errActivityDoesNotExist matches AWS's DescribeActivity/GetActivityTask
// error code for an unknown activity ARN.
const errActivityDoesNotExist = "ActivityDoesNotExist"

// Activity represents a Step Functions activity: a task type hosted and
// polled for by a worker via GetActivityTask, as opposed to an optimized
// service integration.
type Activity struct {
	ActivityArn  string
	Name         string
	CreationDate time.Time
}

// activityARN builds an activity's ARN, matching the same
// arn:aws:states:{region}:{account}:{resource-type}:{name} shape
// CreateStateMachine already uses for state machines (see storage.go).
func (s *MemoryStorage) activityARN(name string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:activity:%s", s.region, s.accountID, name)
}

// CreateActivity creates an activity. It is idempotent on name: a repeat
// call returns the existing ActivityArn/CreationDate unchanged and ignores
// any different Tags.
func (s *MemoryStorage) CreateActivity(_ context.Context, name string, tags []Tag) (*Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	arn := s.activityARN(name)

	if existing, ok := s.Activities[arn]; ok {
		return existing, nil
	}

	activity := &Activity{
		ActivityArn:  arn,
		Name:         name,
		CreationDate: time.Now(),
	}

	s.Activities[arn] = activity

	if len(tags) > 0 {
		s.Tags[arn] = append([]Tag{}, tags...)
	}

	s.saveLocked()

	return activity, nil
}

// DescribeActivity describes an activity.
func (s *MemoryStorage) DescribeActivity(_ context.Context, arn string) (*Activity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activity, ok := s.Activities[arn]
	if !ok {
		return nil, &ServiceError{Code: errActivityDoesNotExist, Message: "Activity does not exist"}
	}

	return activity, nil
}

// ListActivities lists all activities, ordered by creation date.
func (s *MemoryStorage) ListActivities(_ context.Context, maxResults int32, _ string) ([]*Activity, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 100
	}

	activities := make([]*Activity, 0, len(s.Activities))
	for _, a := range s.Activities {
		activities = append(activities, a)
	}

	sort.Slice(activities, func(i, j int) bool {
		return activities[i].CreationDate.Before(activities[j].CreationDate)
	})

	if limit := int(maxResults); len(activities) > limit {
		activities = activities[:limit]
	}

	return activities, "", nil
}

// DeleteActivity deletes an activity. Per AWS's documented errors for this
// action (only InvalidArn), deleting an ARN that does not exist is not an
// error.
func (s *MemoryStorage) DeleteActivity(_ context.Context, arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Activities, arn)
	delete(s.Tags, arn)

	s.saveLocked()

	return nil
}
