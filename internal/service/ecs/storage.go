package ecs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultAccountID = "000000000000"
	defaultRegion    = "us-east-1"

	// Status constants.
	statusActive   = "ACTIVE"
	statusInactive = "INACTIVE"
	statusRunning  = "RUNNING"
	statusStopped  = "STOPPED"
	statusDraining = "DRAINING"
	statusPending  = "PENDING"
	statusPrimary  = "PRIMARY"
)

// Storage defines the interface for ECS storage operations.
type Storage interface {
	CreateCluster(ctx context.Context, req *CreateClusterRequest) (*Cluster, error)
	DeleteCluster(ctx context.Context, cluster string) (*Cluster, error)
	DescribeClusters(ctx context.Context, clusters []string) ([]Cluster, []Failure, error)
	ListClusters(ctx context.Context, maxResults int, nextToken string) ([]string, string, error)

	RegisterTaskDefinition(ctx context.Context, req *RegisterTaskDefinitionRequest) (*TaskDefinition, error)
	DeregisterTaskDefinition(ctx context.Context, taskDefinition string) (*TaskDefinition, error)

	RunTask(ctx context.Context, req *RunTaskRequest) ([]Task, []Failure, error)
	StopTask(ctx context.Context, cluster, task, reason string) (*Task, error)
	DescribeTasks(ctx context.Context, cluster string, tasks []string) ([]Task, []Failure, error)

	CreateService(ctx context.Context, req *CreateServiceRequest) (*ServiceResource, error)
	DeleteService(ctx context.Context, cluster, service string, force bool) (*ServiceResource, error)
	UpdateService(ctx context.Context, req *UpdateServiceRequest) (*ServiceResource, error)
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu              sync.RWMutex                `json:"-"`
	Clusters        map[string]*Cluster         `json:"clusters"`
	TaskDefinitions map[string]*TaskDefinition  `json:"taskDefinitions"`
	TaskDefFamilies map[string][]string         `json:"taskDefFamilies"`
	Tasks           map[string]*Task            `json:"tasks"`
	Services        map[string]*ServiceResource `json:"services"`
	region          string
	dataDir         string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Clusters:        make(map[string]*Cluster),
		TaskDefinitions: make(map[string]*TaskDefinition),
		TaskDefFamilies: make(map[string][]string),
		Tasks:           make(map[string]*Task),
		Services:        make(map[string]*ServiceResource),
		region:          region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "ecs", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(m)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.Clusters == nil {
		m.Clusters = make(map[string]*Cluster)
	}

	if m.TaskDefinitions == nil {
		m.TaskDefinitions = make(map[string]*TaskDefinition)
	}

	if m.TaskDefFamilies == nil {
		m.TaskDefFamilies = make(map[string][]string)
	}

	if m.Tasks == nil {
		m.Tasks = make(map[string]*Task)
	}

	if m.Services == nil {
		m.Services = make(map[string]*ServiceResource)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(m)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(m.dataDir, "ecs", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "ecs", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

func generateID() string {
	return uuid.New().String()[:8]
}

func newTimestamp() *Timestamp {
	return &Timestamp{Time: time.Now()}
}

func (m *MemoryStorage) clusterArn(name string) string {
	return fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", m.region, defaultAccountID, name)
}

func (m *MemoryStorage) taskDefinitionArn(family string, revision int) string {
	return fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/%s:%d", m.region, defaultAccountID, family, revision)
}

func (m *MemoryStorage) taskArn(clusterName, taskID string) string {
	return fmt.Sprintf("arn:aws:ecs:%s:%s:task/%s/%s", m.region, defaultAccountID, clusterName, taskID)
}

func (m *MemoryStorage) serviceArn(clusterName, serviceName string) string {
	return fmt.Sprintf("arn:aws:ecs:%s:%s:service/%s/%s", m.region, defaultAccountID, clusterName, serviceName)
}

// CreateCluster creates a new ECS cluster.
func (m *MemoryStorage) CreateCluster(_ context.Context, req *CreateClusterRequest) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := req.ClusterName
	if name == "" {
		name = "default"
	}

	arn := m.clusterArn(name)

	// Check if cluster already exists.
	if existing, ok := m.Clusters[arn]; ok {
		return existing, nil
	}

	cluster := &Cluster{
		ClusterArn:  arn,
		ClusterName: name,
		Status:      statusActive,
		Tags:        req.Tags,
	}

	m.Clusters[arn] = cluster

	m.saveLocked()

	return cluster, nil
}

// DeleteCluster deletes an ECS cluster.
func (m *MemoryStorage) DeleteCluster(_ context.Context, cluster string) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	arn := m.resolveClusterArn(cluster)

	existing, ok := m.Clusters[arn]
	if !ok {
		return nil, &Error{
			Code:    "ClusterNotFoundException",
			Message: "The specified cluster was not found",
		}
	}

	// Check if cluster has active services or tasks.
	if existing.ActiveServicesCount > 0 {
		return nil, &Error{
			Code:    "ClusterContainsServicesException",
			Message: "The cluster contains active services",
		}
	}

	if existing.RunningTasksCount > 0 {
		return nil, &Error{
			Code:    "ClusterContainsTasksException",
			Message: "The cluster contains running tasks",
		}
	}

	existing.Status = statusInactive

	delete(m.Clusters, arn)

	m.saveLocked()

	return existing, nil
}

// DescribeClusters describes ECS clusters.
func (m *MemoryStorage) DescribeClusters(_ context.Context, clusters []string) ([]Cluster, []Failure, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var (
		result   []Cluster
		failures []Failure
	)

	// If no clusters specified, return all.
	if len(clusters) == 0 {
		for _, c := range m.Clusters {
			result = append(result, *c)
		}

		return result, failures, nil
	}

	for _, cluster := range clusters {
		arn := m.resolveClusterArn(cluster)

		if c, ok := m.Clusters[arn]; ok {
			result = append(result, *c)
		} else {
			failures = append(failures, Failure{
				Arn:    arn,
				Reason: "MISSING",
			})
		}
	}

	return result, failures, nil
}

// ListClusters lists ECS cluster ARNs.
func (m *MemoryStorage) ListClusters(_ context.Context, _ int, _ string) ([]string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arns := make([]string, 0, len(m.Clusters))
	for arn := range m.Clusters {
		arns = append(arns, arn)
	}

	return arns, "", nil
}

// RegisterTaskDefinition registers a new task definition.
func (m *MemoryStorage) RegisterTaskDefinition(_ context.Context, req *RegisterTaskDefinitionRequest) (*TaskDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Determine revision number.
	revision := 1
	if existing, ok := m.TaskDefFamilies[req.Family]; ok {
		revision = len(existing) + 1
	}

	arn := m.taskDefinitionArn(req.Family, revision)

	td := &TaskDefinition{
		TaskDefinitionArn:       arn,
		Family:                  req.Family,
		Revision:                revision,
		Status:                  statusActive,
		ContainerDefinitions:    req.ContainerDefinitions,
		CPU:                     req.CPU,
		Memory:                  req.Memory,
		NetworkMode:             req.NetworkMode,
		RequiresCompatibilities: req.RequiresCompatibilities,
		ExecutionRoleArn:        req.ExecutionRoleArn,
		TaskRoleArn:             req.TaskRoleArn,
		Tags:                    req.Tags,
	}

	m.TaskDefinitions[arn] = td
	m.TaskDefFamilies[req.Family] = append(m.TaskDefFamilies[req.Family], arn)

	m.saveLocked()

	return td, nil
}

// DeregisterTaskDefinition deregisters a task definition.
func (m *MemoryStorage) DeregisterTaskDefinition(_ context.Context, taskDefinition string) (*TaskDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	arn := m.resolveTaskDefinitionArn(taskDefinition)

	td, ok := m.TaskDefinitions[arn]
	if !ok {
		return nil, &Error{
			Code:    "ClientException",
			Message: "The specified task definition was not found",
		}
	}

	td.Status = statusInactive

	m.saveLocked()

	return td, nil
}

// RunTask runs a task.
func (m *MemoryStorage) RunTask(_ context.Context, req *RunTaskRequest) ([]Task, []Failure, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusterArn := m.resolveClusterArn(req.Cluster)

	cluster, err := m.getOrCreateCluster(clusterArn, req.Cluster)
	if err != nil {
		return nil, nil, err
	}

	td, err := m.getTaskDefinitionForRun(req.TaskDefinition)
	if err != nil {
		return nil, nil, err
	}

	count := req.Count
	if count == 0 {
		count = 1
	}

	launchType := req.LaunchType
	if launchType == "" {
		launchType = "EC2"
	}

	tasks := m.createTasks(clusterArn, td, req, count, launchType)
	cluster.RunningTasksCount += count

	m.saveLocked()

	return tasks, nil, nil
}

func (m *MemoryStorage) getOrCreateCluster(clusterArn, clusterName string) (*Cluster, error) {
	cluster, ok := m.Clusters[clusterArn]
	if ok {
		return cluster, nil
	}

	if clusterName == "" || clusterName == "default" {
		cluster = &Cluster{
			ClusterArn:  clusterArn,
			ClusterName: "default",
			Status:      statusActive,
		}
		m.Clusters[clusterArn] = cluster

		return cluster, nil
	}

	return nil, &Error{
		Code:    "ClusterNotFoundException",
		Message: "The specified cluster was not found",
	}
}

func (m *MemoryStorage) getTaskDefinitionForRun(taskDef string) (*TaskDefinition, error) {
	tdArn := m.resolveTaskDefinitionArn(taskDef)

	td, ok := m.TaskDefinitions[tdArn]
	if !ok {
		return nil, &Error{
			Code:    "ClientException",
			Message: "The specified task definition was not found",
		}
	}

	return td, nil
}

func (m *MemoryStorage) createTasks(clusterArn string, td *TaskDefinition, req *RunTaskRequest, count int, launchType string) []Task {
	tasks := make([]Task, 0, count)
	tdArn := td.TaskDefinitionArn

	for range count {
		task := m.createSingleTask(clusterArn, td, tdArn, req, launchType)
		m.Tasks[task.TaskArn] = &task
		tasks = append(tasks, task)
	}

	return tasks
}

func (m *MemoryStorage) createSingleTask(clusterArn string, td *TaskDefinition, tdArn string, req *RunTaskRequest, launchType string) Task {
	taskID := generateID()
	containers := m.createContainersFromDefinitions(td.ContainerDefinitions)
	clusterName := extractClusterName(clusterArn)

	return Task{
		TaskArn:           m.taskArn(clusterName, taskID),
		ClusterArn:        clusterArn,
		TaskDefinitionArn: tdArn,
		LastStatus:        statusRunning,
		DesiredStatus:     statusRunning,
		CPU:               td.CPU,
		Memory:            td.Memory,
		Containers:        containers,
		StartedAt:         newTimestamp(),
		Group:             req.Group,
		LaunchType:        launchType,
		Tags:              req.Tags,
	}
}

func (m *MemoryStorage) createContainersFromDefinitions(defs []ContainerDefinition) []Container {
	containers := make([]Container, 0, len(defs))

	for i := range defs {
		containers = append(containers, Container{
			ContainerArn: fmt.Sprintf("arn:aws:ecs:%s:%s:container/%s", m.region, defaultAccountID, generateID()),
			Name:         defs[i].Name,
			Image:        defs[i].Image,
			LastStatus:   statusRunning,
		})
	}

	return containers
}

// StopTask stops a running task.
func (m *MemoryStorage) StopTask(_ context.Context, cluster, taskID, reason string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusterArn := m.resolveClusterArn(cluster)

	task, ok := m.Tasks[taskID]
	if !ok {
		// Try to find by short ID.
		for arn, t := range m.Tasks {
			if strings.HasSuffix(arn, "/"+taskID) && t.ClusterArn == clusterArn {
				task = t

				break
			}
		}
	}

	if task == nil {
		return nil, &Error{
			Code:    "InvalidParameterException",
			Message: "The specified task was not found",
		}
	}

	task.LastStatus = statusStopped
	task.DesiredStatus = statusStopped
	task.StoppedAt = newTimestamp()
	task.StoppedReason = reason

	for i := range task.Containers {
		task.Containers[i].LastStatus = statusStopped
	}

	// Update cluster task count.
	if c, ok := m.Clusters[task.ClusterArn]; ok && c.RunningTasksCount > 0 {
		c.RunningTasksCount--
	}

	m.saveLocked()

	return task, nil
}

// DescribeTasks describes tasks.
func (m *MemoryStorage) DescribeTasks(_ context.Context, cluster string, taskIDs []string) ([]Task, []Failure, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusterArn := m.resolveClusterArn(cluster)

	var (
		tasks    []Task
		failures []Failure
	)

	for _, taskID := range taskIDs {
		found := false

		for arn, task := range m.Tasks {
			if task.ClusterArn != clusterArn {
				continue
			}

			if arn == taskID || strings.HasSuffix(arn, "/"+taskID) {
				tasks = append(tasks, *task)
				found = true

				break
			}
		}

		if !found {
			failures = append(failures, Failure{
				Arn:    taskID,
				Reason: "MISSING",
			})
		}
	}

	return tasks, failures, nil
}

// CreateService creates an ECS service.
func (m *MemoryStorage) CreateService(_ context.Context, req *CreateServiceRequest) (*ServiceResource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusterArn := m.resolveClusterArn(req.Cluster)

	cluster, ok := m.Clusters[clusterArn]
	if !ok {
		return nil, &Error{
			Code:    "ClusterNotFoundException",
			Message: "The specified cluster was not found",
		}
	}

	clusterName := extractClusterName(clusterArn)
	arn := m.serviceArn(clusterName, req.ServiceName)

	// Check if service already exists.
	if _, ok := m.Services[arn]; ok {
		return nil, &Error{
			Code:    "ServiceAlreadyExistsException",
			Message: "A service with that name already exists",
		}
	}

	launchType := req.LaunchType
	if launchType == "" {
		launchType = "EC2"
	}

	ts := newTimestamp()
	svc := &ServiceResource{
		ServiceArn:     arn,
		ServiceName:    req.ServiceName,
		ClusterArn:     clusterArn,
		TaskDefinition: m.resolveTaskDefinitionArn(req.TaskDefinition),
		DesiredCount:   req.DesiredCount,
		RunningCount:   0,
		PendingCount:   req.DesiredCount,
		LaunchType:     launchType,
		Status:         statusActive,
		Deployments: []Deployment{
			{
				ID:             generateID(),
				Status:         statusPrimary,
				TaskDefinition: m.resolveTaskDefinitionArn(req.TaskDefinition),
				DesiredCount:   req.DesiredCount,
				RunningCount:   0,
				PendingCount:   req.DesiredCount,
				CreatedAt:      ts,
				UpdatedAt:      ts,
			},
		},
		Tags: req.Tags,
	}

	m.Services[arn] = svc
	cluster.ActiveServicesCount++

	m.saveLocked()

	return svc, nil
}

// DeleteService deletes an ECS service.
func (m *MemoryStorage) DeleteService(_ context.Context, cluster, service string, force bool) (*ServiceResource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusterArn := m.resolveClusterArn(cluster)
	clusterName := extractClusterName(clusterArn)
	svcArn := m.serviceArn(clusterName, service)

	// Try to find by ARN or name.
	svc, ok := m.Services[svcArn]
	if !ok {
		// Try to find by full ARN.
		svc, ok = m.Services[service]
		if !ok {
			return nil, &Error{
				Code:    "ServiceNotFoundException",
				Message: "The specified service was not found",
			}
		}

		svcArn = service
	}

	if svc.RunningCount > 0 && !force {
		return nil, &Error{
			Code:    "ServiceNotDrainedException",
			Message: "The service still has running tasks",
		}
	}

	svc.Status = statusDraining

	delete(m.Services, svcArn)

	// Update cluster service count.
	if c, ok := m.Clusters[svc.ClusterArn]; ok && c.ActiveServicesCount > 0 {
		c.ActiveServicesCount--
	}

	m.saveLocked()

	return svc, nil
}

// UpdateService updates an ECS service.
func (m *MemoryStorage) UpdateService(_ context.Context, req *UpdateServiceRequest) (*ServiceResource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusterArn := m.resolveClusterArn(req.Cluster)
	clusterName := extractClusterName(clusterArn)
	svcArn := m.serviceArn(clusterName, req.Service)

	// Try to find by ARN or name.
	svc, ok := m.Services[svcArn]
	if !ok {
		svc, ok = m.Services[req.Service]
		if !ok {
			return nil, &Error{
				Code:    "ServiceNotFoundException",
				Message: "The specified service was not found",
			}
		}
	}

	if req.TaskDefinition != "" {
		svc.TaskDefinition = m.resolveTaskDefinitionArn(req.TaskDefinition)
	}

	if req.DesiredCount != nil {
		svc.DesiredCount = *req.DesiredCount
		svc.PendingCount = max(*req.DesiredCount-svc.RunningCount, 0)
	}

	// Update deployment.
	if len(svc.Deployments) > 0 {
		svc.Deployments[0].TaskDefinition = svc.TaskDefinition
		svc.Deployments[0].DesiredCount = svc.DesiredCount
		svc.Deployments[0].UpdatedAt = newTimestamp()
	}

	m.saveLocked()

	return svc, nil
}

// Helper methods.

func (m *MemoryStorage) resolveClusterArn(cluster string) string {
	if cluster == "" {
		return m.clusterArn("default")
	}

	if strings.HasPrefix(cluster, "arn:") {
		return cluster
	}

	return m.clusterArn(cluster)
}

func (m *MemoryStorage) resolveTaskDefinitionArn(taskDefinition string) string {
	if strings.HasPrefix(taskDefinition, "arn:") {
		return taskDefinition
	}

	// Try family:revision format.
	parts := strings.Split(taskDefinition, ":")
	if len(parts) == 2 {
		return fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/%s", m.region, defaultAccountID, taskDefinition)
	}

	// Try to find latest revision.
	if arns, ok := m.TaskDefFamilies[taskDefinition]; ok && len(arns) > 0 {
		return arns[len(arns)-1]
	}

	return fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/%s:1", m.region, defaultAccountID, taskDefinition)
}

func extractClusterName(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}

	return arn
}
