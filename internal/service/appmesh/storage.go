package appmesh

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultAccountID = "123456789012"
	defaultRegion    = "us-east-1"
)

// Storage defines the interface for App Mesh storage operations.
type Storage interface {
	// Mesh operations.
	CreateMesh(ctx context.Context, req *CreateMeshInput) (*MeshData, error)
	DescribeMesh(ctx context.Context, meshName string) (*MeshData, error)
	ListMeshes(ctx context.Context, req *ListMeshesInput) (*ListMeshesOutput, error)
	UpdateMesh(ctx context.Context, req *UpdateMeshInput) (*MeshData, error)
	DeleteMesh(ctx context.Context, meshName string) (*MeshData, error)

	// Virtual Node operations.
	CreateVirtualNode(ctx context.Context, req *CreateVirtualNodeInput) (*VirtualNodeData, error)
	DescribeVirtualNode(ctx context.Context, meshName, virtualNodeName string) (*VirtualNodeData, error)
	ListVirtualNodes(ctx context.Context, req *ListVirtualNodesInput) (*ListVirtualNodesOutput, error)
	UpdateVirtualNode(ctx context.Context, req *UpdateVirtualNodeInput) (*VirtualNodeData, error)
	DeleteVirtualNode(ctx context.Context, meshName, virtualNodeName string) (*VirtualNodeData, error)

	// Virtual Service operations.
	CreateVirtualService(ctx context.Context, req *CreateVirtualServiceInput) (*VirtualServiceData, error)
	DescribeVirtualService(ctx context.Context, meshName, virtualServiceName string) (*VirtualServiceData, error)
	ListVirtualServices(ctx context.Context, req *ListVirtualServicesInput) (*ListVirtualServicesOutput, error)
	UpdateVirtualService(ctx context.Context, req *UpdateVirtualServiceInput) (*VirtualServiceData, error)
	DeleteVirtualService(ctx context.Context, meshName, virtualServiceName string) (*VirtualServiceData, error)

	// Virtual Router operations.
	CreateVirtualRouter(ctx context.Context, req *CreateVirtualRouterInput) (*VirtualRouterData, error)
	DescribeVirtualRouter(ctx context.Context, meshName, virtualRouterName string) (*VirtualRouterData, error)
	ListVirtualRouters(ctx context.Context, req *ListVirtualRoutersInput) (*ListVirtualRoutersOutput, error)
	UpdateVirtualRouter(ctx context.Context, req *UpdateVirtualRouterInput) (*VirtualRouterData, error)
	DeleteVirtualRouter(ctx context.Context, meshName, virtualRouterName string) (*VirtualRouterData, error)

	// Route operations.
	CreateRoute(ctx context.Context, req *CreateRouteInput) (*RouteData, error)
	DescribeRoute(ctx context.Context, meshName, virtualRouterName, routeName string) (*RouteData, error)
	ListRoutes(ctx context.Context, req *ListRoutesInput) (*ListRoutesOutput, error)
	UpdateRoute(ctx context.Context, req *UpdateRouteInput) (*RouteData, error)
	DeleteRoute(ctx context.Context, meshName, virtualRouterName, routeName string) (*RouteData, error)
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

// MemoryStorage implements Storage with in-memory storage.
type MemoryStorage struct {
	mu              sync.RWMutex                                `json:"-"`
	Meshes          map[string]*MeshData                        `json:"meshes"`
	VirtualNodes    map[string]map[string]*VirtualNodeData      `json:"virtualNodes"`    // meshName -> virtualNodeName -> data
	VirtualServices map[string]map[string]*VirtualServiceData   `json:"virtualServices"` // meshName -> virtualServiceName -> data
	VirtualRouters  map[string]map[string]*VirtualRouterData    `json:"virtualRouters"`  // meshName -> virtualRouterName -> data
	Routes          map[string]map[string]map[string]*RouteData `json:"routes"`          // meshName -> virtualRouterName -> routeName -> data
	region          string
	dataDir         string
}

// NewMemoryStorage creates a new MemoryStorage instance.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Meshes:          make(map[string]*MeshData),
		VirtualNodes:    make(map[string]map[string]*VirtualNodeData),
		VirtualServices: make(map[string]map[string]*VirtualServiceData),
		VirtualRouters:  make(map[string]map[string]*VirtualRouterData),
		Routes:          make(map[string]map[string]map[string]*RouteData),
		region:          region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "appmesh", s)
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

	if m.Meshes == nil {
		m.Meshes = make(map[string]*MeshData)
	}

	if m.VirtualNodes == nil {
		m.VirtualNodes = make(map[string]map[string]*VirtualNodeData)
	}

	if m.VirtualServices == nil {
		m.VirtualServices = make(map[string]map[string]*VirtualServiceData)
	}

	if m.VirtualRouters == nil {
		m.VirtualRouters = make(map[string]map[string]*VirtualRouterData)
	}

	if m.Routes == nil {
		m.Routes = make(map[string]map[string]map[string]*RouteData)
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

	_ = storage.SaveBytes(m.dataDir, "appmesh", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "appmesh", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// --- Mesh Operations ---

// CreateMesh creates a new mesh.
func (m *MemoryStorage) CreateMesh(_ context.Context, req *CreateMeshInput) (*MeshData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; exists {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Mesh %s already exists", req.MeshName),
		}
	}

	now := time.Now()
	uid := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:appmesh:%s:%s:mesh/%s", m.region, defaultAccountID, req.MeshName)

	mesh := &MeshData{
		MeshName: req.MeshName,
		Metadata: ResourceMetadata{
			Arn:           arn,
			CreatedAt:     AWSTimestamp{now},
			LastUpdatedAt: AWSTimestamp{now},
			MeshOwner:     defaultAccountID,
			ResourceOwner: defaultAccountID,
			UID:           uid,
			Version:       1,
		},
		Spec:   req.Spec,
		Status: ResourceStatus{Status: StatusActive},
	}

	m.Meshes[req.MeshName] = mesh
	m.VirtualNodes[req.MeshName] = make(map[string]*VirtualNodeData)
	m.VirtualServices[req.MeshName] = make(map[string]*VirtualServiceData)
	m.VirtualRouters[req.MeshName] = make(map[string]*VirtualRouterData)
	m.Routes[req.MeshName] = make(map[string]map[string]*RouteData)

	m.saveLocked()

	return mesh, nil
}

// DescribeMesh retrieves a mesh by name.
func (m *MemoryStorage) DescribeMesh(_ context.Context, meshName string) (*MeshData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mesh, exists := m.Meshes[meshName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	return mesh, nil
}

// ListMeshes lists all meshes.
func (m *MemoryStorage) ListMeshes(_ context.Context, req *ListMeshesInput) (*ListMeshesOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	meshRefs := make([]MeshRef, 0, len(m.Meshes))

	for _, mesh := range m.Meshes {
		meshRefs = append(meshRefs, MeshRef{
			Arn:           mesh.Metadata.Arn,
			CreatedAt:     mesh.Metadata.CreatedAt,
			LastUpdatedAt: mesh.Metadata.LastUpdatedAt,
			MeshName:      mesh.MeshName,
			MeshOwner:     mesh.Metadata.MeshOwner,
			ResourceOwner: mesh.Metadata.ResourceOwner,
			Version:       mesh.Metadata.Version,
		})
	}

	if len(meshRefs) > limit {
		meshRefs = meshRefs[:limit]
	}

	return &ListMeshesOutput{
		Meshes: meshRefs,
	}, nil
}

// UpdateMesh updates an existing mesh.
func (m *MemoryStorage) UpdateMesh(_ context.Context, req *UpdateMeshInput) (*MeshData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mesh, exists := m.Meshes[req.MeshName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	mesh.Spec = req.Spec
	mesh.Metadata.LastUpdatedAt = AWSTimestamp{time.Now()}
	mesh.Metadata.Version++

	m.saveLocked()

	return mesh, nil
}

// DeleteMesh deletes a mesh.
func (m *MemoryStorage) DeleteMesh(_ context.Context, meshName string) (*MeshData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mesh, exists := m.Meshes[meshName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	// Check if mesh has any child resources.
	if len(m.VirtualNodes[meshName]) > 0 {
		return nil, &Error{
			Code:    errResourceInUseException,
			Message: fmt.Sprintf("Mesh %s has virtual nodes and cannot be deleted", meshName),
		}
	}

	if len(m.VirtualServices[meshName]) > 0 {
		return nil, &Error{
			Code:    errResourceInUseException,
			Message: fmt.Sprintf("Mesh %s has virtual services and cannot be deleted", meshName),
		}
	}

	if len(m.VirtualRouters[meshName]) > 0 {
		return nil, &Error{
			Code:    errResourceInUseException,
			Message: fmt.Sprintf("Mesh %s has virtual routers and cannot be deleted", meshName),
		}
	}

	mesh.Status.Status = StatusDeleted

	delete(m.Meshes, meshName)
	delete(m.VirtualNodes, meshName)
	delete(m.VirtualServices, meshName)
	delete(m.VirtualRouters, meshName)
	delete(m.Routes, meshName)

	m.saveLocked()

	return mesh, nil
}

// --- Virtual Node Operations ---

// CreateVirtualNode creates a new virtual node.
func (m *MemoryStorage) CreateVirtualNode(_ context.Context, req *CreateVirtualNodeInput) (*VirtualNodeData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	if _, exists := m.VirtualNodes[req.MeshName][req.VirtualNodeName]; exists {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("VirtualNode %s already exists in mesh %s", req.VirtualNodeName, req.MeshName),
		}
	}

	now := time.Now()
	uid := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:appmesh:%s:%s:mesh/%s/virtualNode/%s",
		m.region, defaultAccountID, req.MeshName, req.VirtualNodeName)

	node := &VirtualNodeData{
		MeshName:        req.MeshName,
		VirtualNodeName: req.VirtualNodeName,
		Metadata: ResourceMetadata{
			Arn:           arn,
			CreatedAt:     AWSTimestamp{now},
			LastUpdatedAt: AWSTimestamp{now},
			MeshOwner:     defaultAccountID,
			ResourceOwner: defaultAccountID,
			UID:           uid,
			Version:       1,
		},
		Spec:   req.Spec,
		Status: ResourceStatus{Status: StatusActive},
	}

	m.VirtualNodes[req.MeshName][req.VirtualNodeName] = node

	m.saveLocked()

	return node, nil
}

// DescribeVirtualNode retrieves a virtual node.
func (m *MemoryStorage) DescribeVirtualNode(_ context.Context, meshName, virtualNodeName string) (*VirtualNodeData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Meshes[meshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	node, exists := m.VirtualNodes[meshName][virtualNodeName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualNode %s not found in mesh %s", virtualNodeName, meshName),
		}
	}

	return node, nil
}

// ListVirtualNodes lists virtual nodes in a mesh.
func (m *MemoryStorage) ListVirtualNodes(_ context.Context, req *ListVirtualNodesInput) (*ListVirtualNodesOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	nodeRefs := make([]VirtualNodeRef, 0)

	for _, node := range m.VirtualNodes[req.MeshName] {
		nodeRefs = append(nodeRefs, VirtualNodeRef{
			Arn:             node.Metadata.Arn,
			CreatedAt:       node.Metadata.CreatedAt,
			LastUpdatedAt:   node.Metadata.LastUpdatedAt,
			MeshName:        node.MeshName,
			MeshOwner:       node.Metadata.MeshOwner,
			ResourceOwner:   node.Metadata.ResourceOwner,
			Version:         node.Metadata.Version,
			VirtualNodeName: node.VirtualNodeName,
		})
	}

	if len(nodeRefs) > limit {
		nodeRefs = nodeRefs[:limit]
	}

	return &ListVirtualNodesOutput{
		VirtualNodes: nodeRefs,
	}, nil
}

// UpdateVirtualNode updates a virtual node.
func (m *MemoryStorage) UpdateVirtualNode(_ context.Context, req *UpdateVirtualNodeInput) (*VirtualNodeData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	node, exists := m.VirtualNodes[req.MeshName][req.VirtualNodeName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualNode %s not found in mesh %s", req.VirtualNodeName, req.MeshName),
		}
	}

	node.Spec = req.Spec
	node.Metadata.LastUpdatedAt = AWSTimestamp{time.Now()}
	node.Metadata.Version++

	m.saveLocked()

	return node, nil
}

// DeleteVirtualNode deletes a virtual node.
func (m *MemoryStorage) DeleteVirtualNode(_ context.Context, meshName, virtualNodeName string) (*VirtualNodeData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[meshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	node, exists := m.VirtualNodes[meshName][virtualNodeName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualNode %s not found in mesh %s", virtualNodeName, meshName),
		}
	}

	node.Status.Status = StatusDeleted

	delete(m.VirtualNodes[meshName], virtualNodeName)

	m.saveLocked()

	return node, nil
}

// --- Virtual Service Operations ---

// CreateVirtualService creates a new virtual service.
func (m *MemoryStorage) CreateVirtualService(_ context.Context, req *CreateVirtualServiceInput) (*VirtualServiceData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	if _, exists := m.VirtualServices[req.MeshName][req.VirtualServiceName]; exists {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("VirtualService %s already exists in mesh %s", req.VirtualServiceName, req.MeshName),
		}
	}

	now := time.Now()
	uid := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:appmesh:%s:%s:mesh/%s/virtualService/%s",
		m.region, defaultAccountID, req.MeshName, req.VirtualServiceName)

	service := &VirtualServiceData{
		MeshName:           req.MeshName,
		VirtualServiceName: req.VirtualServiceName,
		Metadata: ResourceMetadata{
			Arn:           arn,
			CreatedAt:     AWSTimestamp{now},
			LastUpdatedAt: AWSTimestamp{now},
			MeshOwner:     defaultAccountID,
			ResourceOwner: defaultAccountID,
			UID:           uid,
			Version:       1,
		},
		Spec:   req.Spec,
		Status: ResourceStatus{Status: StatusActive},
	}

	m.VirtualServices[req.MeshName][req.VirtualServiceName] = service

	m.saveLocked()

	return service, nil
}

// DescribeVirtualService retrieves a virtual service.
func (m *MemoryStorage) DescribeVirtualService(_ context.Context, meshName, virtualServiceName string) (*VirtualServiceData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Meshes[meshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	service, exists := m.VirtualServices[meshName][virtualServiceName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualService %s not found in mesh %s", virtualServiceName, meshName),
		}
	}

	return service, nil
}

// ListVirtualServices lists virtual services in a mesh.
func (m *MemoryStorage) ListVirtualServices(_ context.Context, req *ListVirtualServicesInput) (*ListVirtualServicesOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	serviceRefs := make([]VirtualServiceRef, 0)

	for _, service := range m.VirtualServices[req.MeshName] {
		serviceRefs = append(serviceRefs, VirtualServiceRef{
			Arn:                service.Metadata.Arn,
			CreatedAt:          service.Metadata.CreatedAt,
			LastUpdatedAt:      service.Metadata.LastUpdatedAt,
			MeshName:           service.MeshName,
			MeshOwner:          service.Metadata.MeshOwner,
			ResourceOwner:      service.Metadata.ResourceOwner,
			Version:            service.Metadata.Version,
			VirtualServiceName: service.VirtualServiceName,
		})
	}

	if len(serviceRefs) > limit {
		serviceRefs = serviceRefs[:limit]
	}

	return &ListVirtualServicesOutput{
		VirtualServices: serviceRefs,
	}, nil
}

// UpdateVirtualService updates a virtual service.
func (m *MemoryStorage) UpdateVirtualService(_ context.Context, req *UpdateVirtualServiceInput) (*VirtualServiceData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	service, exists := m.VirtualServices[req.MeshName][req.VirtualServiceName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualService %s not found in mesh %s", req.VirtualServiceName, req.MeshName),
		}
	}

	service.Spec = req.Spec
	service.Metadata.LastUpdatedAt = AWSTimestamp{time.Now()}
	service.Metadata.Version++

	m.saveLocked()

	return service, nil
}

// DeleteVirtualService deletes a virtual service.
func (m *MemoryStorage) DeleteVirtualService(_ context.Context, meshName, virtualServiceName string) (*VirtualServiceData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[meshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	service, exists := m.VirtualServices[meshName][virtualServiceName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualService %s not found in mesh %s", virtualServiceName, meshName),
		}
	}

	service.Status.Status = StatusDeleted

	delete(m.VirtualServices[meshName], virtualServiceName)

	m.saveLocked()

	return service, nil
}

// --- Virtual Router Operations ---

// CreateVirtualRouter creates a new virtual router.
func (m *MemoryStorage) CreateVirtualRouter(_ context.Context, req *CreateVirtualRouterInput) (*VirtualRouterData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	if _, exists := m.VirtualRouters[req.MeshName][req.VirtualRouterName]; exists {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("VirtualRouter %s already exists in mesh %s", req.VirtualRouterName, req.MeshName),
		}
	}

	now := time.Now()
	uid := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:appmesh:%s:%s:mesh/%s/virtualRouter/%s",
		m.region, defaultAccountID, req.MeshName, req.VirtualRouterName)

	router := &VirtualRouterData{
		MeshName:          req.MeshName,
		VirtualRouterName: req.VirtualRouterName,
		Metadata: ResourceMetadata{
			Arn:           arn,
			CreatedAt:     AWSTimestamp{now},
			LastUpdatedAt: AWSTimestamp{now},
			MeshOwner:     defaultAccountID,
			ResourceOwner: defaultAccountID,
			UID:           uid,
			Version:       1,
		},
		Spec:   req.Spec,
		Status: ResourceStatus{Status: StatusActive},
	}

	m.VirtualRouters[req.MeshName][req.VirtualRouterName] = router
	m.Routes[req.MeshName][req.VirtualRouterName] = make(map[string]*RouteData)

	m.saveLocked()

	return router, nil
}

// DescribeVirtualRouter retrieves a virtual router.
func (m *MemoryStorage) DescribeVirtualRouter(_ context.Context, meshName, virtualRouterName string) (*VirtualRouterData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Meshes[meshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	router, exists := m.VirtualRouters[meshName][virtualRouterName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualRouter %s not found in mesh %s", virtualRouterName, meshName),
		}
	}

	return router, nil
}

// ListVirtualRouters lists virtual routers in a mesh.
func (m *MemoryStorage) ListVirtualRouters(_ context.Context, req *ListVirtualRoutersInput) (*ListVirtualRoutersOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	routerRefs := make([]VirtualRouterRef, 0)

	for _, router := range m.VirtualRouters[req.MeshName] {
		routerRefs = append(routerRefs, VirtualRouterRef{
			Arn:               router.Metadata.Arn,
			CreatedAt:         router.Metadata.CreatedAt,
			LastUpdatedAt:     router.Metadata.LastUpdatedAt,
			MeshName:          router.MeshName,
			MeshOwner:         router.Metadata.MeshOwner,
			ResourceOwner:     router.Metadata.ResourceOwner,
			Version:           router.Metadata.Version,
			VirtualRouterName: router.VirtualRouterName,
		})
	}

	if len(routerRefs) > limit {
		routerRefs = routerRefs[:limit]
	}

	return &ListVirtualRoutersOutput{
		VirtualRouters: routerRefs,
	}, nil
}

// UpdateVirtualRouter updates a virtual router.
func (m *MemoryStorage) UpdateVirtualRouter(_ context.Context, req *UpdateVirtualRouterInput) (*VirtualRouterData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	router, exists := m.VirtualRouters[req.MeshName][req.VirtualRouterName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualRouter %s not found in mesh %s", req.VirtualRouterName, req.MeshName),
		}
	}

	router.Spec = req.Spec
	router.Metadata.LastUpdatedAt = AWSTimestamp{time.Now()}
	router.Metadata.Version++

	m.saveLocked()

	return router, nil
}

// DeleteVirtualRouter deletes a virtual router.
func (m *MemoryStorage) DeleteVirtualRouter(_ context.Context, meshName, virtualRouterName string) (*VirtualRouterData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[meshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	router, exists := m.VirtualRouters[meshName][virtualRouterName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualRouter %s not found in mesh %s", virtualRouterName, meshName),
		}
	}

	// Check if router has any routes.
	if len(m.Routes[meshName][virtualRouterName]) > 0 {
		return nil, &Error{
			Code:    errResourceInUseException,
			Message: fmt.Sprintf("VirtualRouter %s has routes and cannot be deleted", virtualRouterName),
		}
	}

	router.Status.Status = StatusDeleted

	delete(m.VirtualRouters[meshName], virtualRouterName)
	delete(m.Routes[meshName], virtualRouterName)

	m.saveLocked()

	return router, nil
}

// --- Route Operations ---

// CreateRoute creates a new route.
func (m *MemoryStorage) CreateRoute(_ context.Context, req *CreateRouteInput) (*RouteData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	if _, exists := m.VirtualRouters[req.MeshName][req.VirtualRouterName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualRouter %s not found in mesh %s", req.VirtualRouterName, req.MeshName),
		}
	}

	if _, exists := m.Routes[req.MeshName][req.VirtualRouterName][req.RouteName]; exists {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Route %s already exists in virtual router %s", req.RouteName, req.VirtualRouterName),
		}
	}

	now := time.Now()
	uid := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:appmesh:%s:%s:mesh/%s/virtualRouter/%s/route/%s",
		m.region, defaultAccountID, req.MeshName, req.VirtualRouterName, req.RouteName)

	route := &RouteData{
		MeshName:          req.MeshName,
		VirtualRouterName: req.VirtualRouterName,
		RouteName:         req.RouteName,
		Metadata: ResourceMetadata{
			Arn:           arn,
			CreatedAt:     AWSTimestamp{now},
			LastUpdatedAt: AWSTimestamp{now},
			MeshOwner:     defaultAccountID,
			ResourceOwner: defaultAccountID,
			UID:           uid,
			Version:       1,
		},
		Spec:   req.Spec,
		Status: ResourceStatus{Status: StatusActive},
	}

	m.Routes[req.MeshName][req.VirtualRouterName][req.RouteName] = route

	m.saveLocked()

	return route, nil
}

// DescribeRoute retrieves a route.
func (m *MemoryStorage) DescribeRoute(_ context.Context, meshName, virtualRouterName, routeName string) (*RouteData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Meshes[meshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	if _, exists := m.VirtualRouters[meshName][virtualRouterName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualRouter %s not found in mesh %s", virtualRouterName, meshName),
		}
	}

	route, exists := m.Routes[meshName][virtualRouterName][routeName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Route %s not found in virtual router %s", routeName, virtualRouterName),
		}
	}

	return route, nil
}

// ListRoutes lists routes in a virtual router.
func (m *MemoryStorage) ListRoutes(_ context.Context, req *ListRoutesInput) (*ListRoutesOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	if _, exists := m.VirtualRouters[req.MeshName][req.VirtualRouterName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualRouter %s not found in mesh %s", req.VirtualRouterName, req.MeshName),
		}
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	routeRefs := make([]RouteRef, 0)

	for _, route := range m.Routes[req.MeshName][req.VirtualRouterName] {
		routeRefs = append(routeRefs, RouteRef{
			Arn:               route.Metadata.Arn,
			CreatedAt:         route.Metadata.CreatedAt,
			LastUpdatedAt:     route.Metadata.LastUpdatedAt,
			MeshName:          route.MeshName,
			MeshOwner:         route.Metadata.MeshOwner,
			ResourceOwner:     route.Metadata.ResourceOwner,
			RouteName:         route.RouteName,
			Version:           route.Metadata.Version,
			VirtualRouterName: route.VirtualRouterName,
		})
	}

	if len(routeRefs) > limit {
		routeRefs = routeRefs[:limit]
	}

	return &ListRoutesOutput{
		Routes: routeRefs,
	}, nil
}

// UpdateRoute updates a route.
func (m *MemoryStorage) UpdateRoute(_ context.Context, req *UpdateRouteInput) (*RouteData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[req.MeshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", req.MeshName),
		}
	}

	if _, exists := m.VirtualRouters[req.MeshName][req.VirtualRouterName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualRouter %s not found in mesh %s", req.VirtualRouterName, req.MeshName),
		}
	}

	route, exists := m.Routes[req.MeshName][req.VirtualRouterName][req.RouteName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Route %s not found in virtual router %s", req.RouteName, req.VirtualRouterName),
		}
	}

	route.Spec = req.Spec
	route.Metadata.LastUpdatedAt = AWSTimestamp{time.Now()}
	route.Metadata.Version++

	m.saveLocked()

	return route, nil
}

// DeleteRoute deletes a route.
func (m *MemoryStorage) DeleteRoute(_ context.Context, meshName, virtualRouterName, routeName string) (*RouteData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Meshes[meshName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Mesh %s not found", meshName),
		}
	}

	if _, exists := m.VirtualRouters[meshName][virtualRouterName]; !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("VirtualRouter %s not found in mesh %s", virtualRouterName, meshName),
		}
	}

	route, exists := m.Routes[meshName][virtualRouterName][routeName]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Route %s not found in virtual router %s", routeName, virtualRouterName),
		}
	}

	route.Status.Status = StatusDeleted

	delete(m.Routes[meshName][virtualRouterName], routeName)

	m.saveLocked()

	return route, nil
}

// Compile-time interface check.
var _ Storage = (*MemoryStorage)(nil)
