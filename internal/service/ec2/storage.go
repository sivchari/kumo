package ec2

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"github.com/sivchari/kumo/internal/storage"
)

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

const defaultAccountID = "000000000000"

// Instance state codes.
const (
	InstanceStatePending      = 0
	InstanceStateRunning      = 16
	InstanceStateShuttingDown = 32
	InstanceStateTerminated   = 48
	InstanceStateStopping     = 64
	InstanceStateStopped      = 80
)

// Instance state names.
const (
	InstanceStateNamePending      = "pending"
	InstanceStateNameRunning      = "running"
	InstanceStateNameShuttingDown = "shutting-down"
	InstanceStateNameTerminated   = "terminated"
	InstanceStateNameStopping     = "stopping"
	InstanceStateNameStopped      = "stopped"
)

// Storage defines the EC2 storage interface.
type Storage interface {
	// Instance operations
	RunInstances(ctx context.Context, req *RunInstancesRequest) ([]*Instance, string, error)
	TerminateInstances(ctx context.Context, instanceIDs []string) ([]InstanceStateChange, error)
	DescribeInstances(ctx context.Context, instanceIDs []string) ([]*Reservation, error)
	StartInstances(ctx context.Context, instanceIDs []string) ([]InstanceStateChange, error)
	StopInstances(ctx context.Context, instanceIDs []string) ([]InstanceStateChange, error)

	// Security Group operations
	CreateSecurityGroup(ctx context.Context, req *CreateSecurityGroupRequest) (*SecurityGroup, error)
	DeleteSecurityGroup(ctx context.Context, groupID, groupName string) error
	AuthorizeSecurityGroupIngress(ctx context.Context, groupID, groupName string, permissions []IPPermission) error
	AuthorizeSecurityGroupEgress(ctx context.Context, groupID string, permissions []IPPermission) error
	DescribeSecurityGroups(ctx context.Context, groupIDs, groupNames []string) ([]*SecurityGroup, error)

	// Key Pair operations
	CreateKeyPair(ctx context.Context, keyName, keyType string) (*KeyPair, error)
	DeleteKeyPair(ctx context.Context, keyName, keyPairID string) error
	DescribeKeyPairs(ctx context.Context, keyNames, keyPairIDs []string) ([]*KeyPair, error)

	// VPC operations
	CreateVpc(ctx context.Context, req *CreateVpcRequest) (*Vpc, error)
	DeleteVpc(ctx context.Context, vpcID string) error
	DescribeVpcs(ctx context.Context, vpcIDs []string) ([]*Vpc, error)
	ModifyVpcAttribute(ctx context.Context, vpcID string, updates VpcAttributeUpdates) error

	// Subnet operations
	CreateSubnet(ctx context.Context, req *CreateSubnetRequest) (*Subnet, error)
	DeleteSubnet(ctx context.Context, subnetID string) error
	DescribeSubnets(ctx context.Context, subnetIDs []string, filters map[string][]string) ([]*Subnet, error)
	ModifySubnetAttribute(ctx context.Context, subnetID string, updates SubnetAttributeUpdates) error

	// Internet Gateway operations
	CreateInternetGateway(ctx context.Context, req *CreateInternetGatewayRequest) (*InternetGateway, error)
	AttachInternetGateway(ctx context.Context, igwID, vpcID string) error
	DescribeInternetGateways(ctx context.Context, igwIDs []string) ([]*InternetGateway, error)

	// Route Table operations
	CreateRouteTable(ctx context.Context, req *CreateRouteTableRequest) (*RouteTable, error)
	CreateRoute(ctx context.Context, req *CreateRouteRequest) error
	AssociateRouteTable(ctx context.Context, req *AssociateRouteTableRequest) (string, error)
	DescribeRouteTables(ctx context.Context, rtbIDs []string) ([]*RouteTable, error)

	// NAT Gateway operations
	CreateNatGateway(ctx context.Context, req *CreateNatGatewayRequest) (*NatGateway, error)
	DescribeNatGateways(ctx context.Context, natgwIDs []string) ([]*NatGateway, error)

	// Tag operations
	CreateTags(ctx context.Context, resourceIDs []string, tags []Tag) error
	DeleteTags(ctx context.Context, resourceIDs []string, tags []Tag) error
	DescribeTags(ctx context.Context, filters map[string][]string) ([]TagDescription, error)
}

// InstanceStateChange represents an instance state change.
type InstanceStateChange struct {
	InstanceID    string
	CurrentState  InstanceState
	PreviousState InstanceState
}

// Reservation represents a group of instances launched together.
type Reservation struct {
	ReservationID string
	OwnerID       string
	Instances     []*Instance
}

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu               sync.RWMutex                `json:"-"`
	Instances        map[string]*Instance        `json:"instances"`
	Reservations     map[string]*Reservation     `json:"reservations"`
	SecurityGroups   map[string]*SecurityGroup   `json:"securityGroups"`
	KeyPairs         map[string]*KeyPair         `json:"keyPairs"`
	Vpcs             map[string]*Vpc             `json:"vpcs"`
	Subnets          map[string]*Subnet          `json:"subnets"`
	InternetGateways map[string]*InternetGateway `json:"internetGateways"`
	RouteTables      map[string]*RouteTable      `json:"routeTables"`
	NatGateways      map[string]*NatGateway      `json:"natGateways"`
	dataDir          string
}

// NewMemoryStorage creates a new in-memory EC2 storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Instances:        make(map[string]*Instance),
		Reservations:     make(map[string]*Reservation),
		SecurityGroups:   make(map[string]*SecurityGroup),
		KeyPairs:         make(map[string]*KeyPair),
		Vpcs:             make(map[string]*Vpc),
		Subnets:          make(map[string]*Subnet),
		InternetGateways: make(map[string]*InternetGateway),
		RouteTables:      make(map[string]*RouteTable),
		NatGateways:      make(map[string]*NatGateway),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "ec2", s)
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

	if m.Instances == nil {
		m.Instances = make(map[string]*Instance)
	}

	if m.Reservations == nil {
		m.Reservations = make(map[string]*Reservation)
	}

	if m.SecurityGroups == nil {
		m.SecurityGroups = make(map[string]*SecurityGroup)
	}

	if m.KeyPairs == nil {
		m.KeyPairs = make(map[string]*KeyPair)
	}

	if m.Vpcs == nil {
		m.Vpcs = make(map[string]*Vpc)
	}

	if m.Subnets == nil {
		m.Subnets = make(map[string]*Subnet)
	}

	if m.InternetGateways == nil {
		m.InternetGateways = make(map[string]*InternetGateway)
	}

	if m.RouteTables == nil {
		m.RouteTables = make(map[string]*RouteTable)
	}

	if m.NatGateways == nil {
		m.NatGateways = make(map[string]*NatGateway)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "ec2", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// RunInstances creates new EC2 instances.
func (m *MemoryStorage) RunInstances(_ context.Context, req *RunInstancesRequest) ([]*Instance, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := max(req.MaxCount, req.MinCount)

	if count <= 0 {
		count = 1
	}

	reservationID := "r-" + generateID()
	instances := make([]*Instance, 0, count)

	for i := 0; i < count; i++ {
		instance := &Instance{
			InstanceID:       "i-" + generateID(),
			ImageID:          req.ImageID,
			InstanceType:     req.InstanceType,
			State:            InstanceState{Code: InstanceStateRunning, Name: InstanceStateNameRunning},
			PrivateIPAddress: generatePrivateIP(),
			KeyName:          req.KeyName,
			LaunchTime:       time.Now(),
			SecurityGroups:   m.resolveSecurityGroups(req.SecurityGroupIDs, req.SecurityGroups),
		}
		instances = append(instances, instance)
		m.Instances[instance.InstanceID] = instance
	}

	m.Reservations[reservationID] = &Reservation{
		ReservationID: reservationID,
		OwnerID:       defaultAccountID,
		Instances:     instances,
	}

	return instances, reservationID, nil
}

// TerminateInstances terminates EC2 instances.
func (m *MemoryStorage) TerminateInstances(_ context.Context, instanceIDs []string) ([]InstanceStateChange, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var changes []InstanceStateChange

	for _, id := range instanceIDs {
		instance, exists := m.Instances[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidInstanceID.NotFound",
				Message: fmt.Sprintf("The instance ID '%s' does not exist", id),
			}
		}

		prevState := instance.State
		instance.State = InstanceState{Code: InstanceStateTerminated, Name: InstanceStateNameTerminated}

		changes = append(changes, InstanceStateChange{
			InstanceID:    id,
			CurrentState:  instance.State,
			PreviousState: prevState,
		})
	}

	return changes, nil
}

// DescribeInstances describes EC2 instances.
func (m *MemoryStorage) DescribeInstances(_ context.Context, instanceIDs []string) ([]*Reservation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(instanceIDs) == 0 {
		reservations := make([]*Reservation, 0, len(m.Reservations))
		for _, r := range m.Reservations {
			reservations = append(reservations, r)
		}

		return reservations, nil
	}

	reservationMap := make(map[string]*Reservation)

	for _, id := range instanceIDs {
		instance, exists := m.Instances[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidInstanceID.NotFound",
				Message: fmt.Sprintf("The instance ID '%s' does not exist", id),
			}
		}

		for resID, res := range m.Reservations {
			for _, inst := range res.Instances {
				if inst.InstanceID == id {
					if _, ok := reservationMap[resID]; !ok {
						reservationMap[resID] = &Reservation{
							ReservationID: res.ReservationID,
							OwnerID:       res.OwnerID,
							Instances:     []*Instance{},
						}
					}

					reservationMap[resID].Instances = append(reservationMap[resID].Instances, instance)
				}
			}
		}
	}

	reservations := make([]*Reservation, 0, len(reservationMap))
	for _, r := range reservationMap {
		reservations = append(reservations, r)
	}

	return reservations, nil
}

// StartInstances starts stopped EC2 instances.
func (m *MemoryStorage) StartInstances(_ context.Context, instanceIDs []string) ([]InstanceStateChange, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var changes []InstanceStateChange

	for _, id := range instanceIDs {
		instance, exists := m.Instances[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidInstanceID.NotFound",
				Message: fmt.Sprintf("The instance ID '%s' does not exist", id),
			}
		}

		prevState := instance.State
		instance.State = InstanceState{Code: InstanceStateRunning, Name: InstanceStateNameRunning}

		changes = append(changes, InstanceStateChange{
			InstanceID:    id,
			CurrentState:  instance.State,
			PreviousState: prevState,
		})
	}

	return changes, nil
}

// StopInstances stops running EC2 instances.
func (m *MemoryStorage) StopInstances(_ context.Context, instanceIDs []string) ([]InstanceStateChange, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var changes []InstanceStateChange

	for _, id := range instanceIDs {
		instance, exists := m.Instances[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidInstanceID.NotFound",
				Message: fmt.Sprintf("The instance ID '%s' does not exist", id),
			}
		}

		prevState := instance.State
		instance.State = InstanceState{Code: InstanceStateStopped, Name: InstanceStateNameStopped}

		changes = append(changes, InstanceStateChange{
			InstanceID:    id,
			CurrentState:  instance.State,
			PreviousState: prevState,
		})
	}

	return changes, nil
}

// CreateSecurityGroup creates a new security group.
func (m *MemoryStorage) CreateSecurityGroup(_ context.Context, req *CreateSecurityGroupRequest) (*SecurityGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sg := range m.SecurityGroups {
		if sg.GroupName == req.GroupName {
			return nil, &Error{
				Code:    "InvalidGroup.Duplicate",
				Message: fmt.Sprintf("The security group '%s' already exists", req.GroupName),
			}
		}
	}

	sg := &SecurityGroup{
		GroupID:      "sg-" + generateID(),
		GroupName:    req.GroupName,
		Description:  req.GroupDescription,
		VpcID:        req.VpcID,
		IngressRules: []IPPermission{},
		EgressRules:  []IPPermission{},
	}

	m.SecurityGroups[sg.GroupID] = sg

	return sg, nil
}

// DeleteSecurityGroup deletes a security group.
func (m *MemoryStorage) DeleteSecurityGroup(_ context.Context, groupID, groupName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if groupID != "" {
		if _, exists := m.SecurityGroups[groupID]; !exists {
			return &Error{
				Code:    "InvalidGroup.NotFound",
				Message: fmt.Sprintf("The security group '%s' does not exist", groupID),
			}
		}

		delete(m.SecurityGroups, groupID)

		return nil
	}

	for id, sg := range m.SecurityGroups {
		if sg.GroupName == groupName {
			delete(m.SecurityGroups, id)

			return nil
		}
	}

	return &Error{
		Code:    "InvalidGroup.NotFound",
		Message: fmt.Sprintf("The security group '%s' does not exist", groupName),
	}
}

// AuthorizeSecurityGroupIngress adds ingress rules to a security group.
func (m *MemoryStorage) AuthorizeSecurityGroupIngress(_ context.Context, groupID, groupName string, permissions []IPPermission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sg := m.findSecurityGroup(groupID, groupName)
	if sg == nil {
		return &Error{
			Code:    "InvalidGroup.NotFound",
			Message: "The security group does not exist",
		}
	}

	sg.IngressRules = append(sg.IngressRules, permissions...)

	return nil
}

// AuthorizeSecurityGroupEgress adds egress rules to a security group.
func (m *MemoryStorage) AuthorizeSecurityGroupEgress(_ context.Context, groupID string, permissions []IPPermission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sg, exists := m.SecurityGroups[groupID]
	if !exists {
		return &Error{
			Code:    "InvalidGroup.NotFound",
			Message: fmt.Sprintf("The security group '%s' does not exist", groupID),
		}
	}

	sg.EgressRules = append(sg.EgressRules, permissions...)

	return nil
}

// DescribeSecurityGroups returns SGs filtered by GroupId / GroupName,
// or every SG when both filters are empty.
func (m *MemoryStorage) DescribeSecurityGroups(_ context.Context, groupIDs, groupNames []string) ([]*SecurityGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(groupIDs) == 0 && len(groupNames) == 0 {
		out := make([]*SecurityGroup, 0, len(m.SecurityGroups))
		for _, sg := range m.SecurityGroups {
			out = append(out, sg)
		}

		return out, nil
	}

	out := make([]*SecurityGroup, 0)

	for _, id := range groupIDs {
		if sg, ok := m.SecurityGroups[id]; ok {
			out = append(out, sg)
		}
	}

	for _, name := range groupNames {
		for _, sg := range m.SecurityGroups {
			if sg.GroupName == name {
				out = append(out, sg)
			}
		}
	}

	return out, nil
}

// CreateKeyPair creates a new key pair.
func (m *MemoryStorage) CreateKeyPair(_ context.Context, keyName, _ string) (*KeyPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, kp := range m.KeyPairs {
		if kp.KeyName == keyName {
			return nil, &Error{
				Code:    "InvalidKeyPair.Duplicate",
				Message: fmt.Sprintf("The keypair '%s' already exists", keyName),
			}
		}
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, &Error{
			Code:    "InternalError",
			Message: "Failed to generate key pair",
		}
	}

	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, &Error{
			Code:    "InternalError",
			Message: "Failed to generate public key",
		}
	}

	fingerprint := generateFingerprint(pubKey.Marshal())

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	kp := &KeyPair{
		KeyName:        keyName,
		KeyFingerprint: fingerprint,
		KeyPairID:      "key-" + generateID(),
		KeyMaterial:    string(privateKeyPEM),
		CreateTime:     time.Now(),
	}

	m.KeyPairs[kp.KeyPairID] = kp

	return kp, nil
}

// DeleteKeyPair deletes a key pair.
func (m *MemoryStorage) DeleteKeyPair(_ context.Context, keyName, keyPairID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if keyPairID != "" {
		if _, exists := m.KeyPairs[keyPairID]; !exists {
			return &Error{
				Code:    "InvalidKeyPair.NotFound",
				Message: fmt.Sprintf("The key pair '%s' does not exist", keyPairID),
			}
		}

		delete(m.KeyPairs, keyPairID)

		return nil
	}

	for id, kp := range m.KeyPairs {
		if kp.KeyName == keyName {
			delete(m.KeyPairs, id)

			return nil
		}
	}

	return &Error{
		Code:    "InvalidKeyPair.NotFound",
		Message: fmt.Sprintf("The key pair '%s' does not exist", keyName),
	}
}

// DescribeKeyPairs describes key pairs.
func (m *MemoryStorage) DescribeKeyPairs(_ context.Context, keyNames, keyPairIDs []string) ([]*KeyPair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(keyNames) == 0 && len(keyPairIDs) == 0 {
		return m.listAllKeyPairs(), nil
	}

	keyPairSet, err := m.collectKeyPairs(keyNames, keyPairIDs)
	if err != nil {
		return nil, err
	}

	return m.keyPairSetToSlice(keyPairSet), nil
}

// listAllKeyPairs returns all key pairs without KeyMaterial.
func (m *MemoryStorage) listAllKeyPairs() []*KeyPair {
	keyPairs := make([]*KeyPair, 0, len(m.KeyPairs))

	for _, kp := range m.KeyPairs {
		keyPairs = append(keyPairs, copyKeyPairInfo(kp))
	}

	return keyPairs
}

// collectKeyPairs collects key pairs by names and IDs.
func (m *MemoryStorage) collectKeyPairs(keyNames, keyPairIDs []string) (map[string]*KeyPair, error) {
	keyPairSet := make(map[string]*KeyPair)

	for _, name := range keyNames {
		kp, err := m.findKeyPairByName(name)
		if err != nil {
			return nil, err
		}

		keyPairSet[kp.KeyPairID] = kp
	}

	for _, id := range keyPairIDs {
		kp, exists := m.KeyPairs[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidKeyPair.NotFound",
				Message: fmt.Sprintf("The key pair '%s' does not exist", id),
			}
		}

		keyPairSet[kp.KeyPairID] = kp
	}

	return keyPairSet, nil
}

// findKeyPairByName finds a key pair by name.
func (m *MemoryStorage) findKeyPairByName(name string) (*KeyPair, error) {
	for _, kp := range m.KeyPairs {
		if kp.KeyName == name {
			return kp, nil
		}
	}

	return nil, &Error{
		Code:    "InvalidKeyPair.NotFound",
		Message: fmt.Sprintf("The key pair '%s' does not exist", name),
	}
}

// keyPairSetToSlice converts a key pair set to a slice without KeyMaterial.
func (m *MemoryStorage) keyPairSetToSlice(keyPairSet map[string]*KeyPair) []*KeyPair {
	keyPairs := make([]*KeyPair, 0, len(keyPairSet))

	for _, kp := range keyPairSet {
		keyPairs = append(keyPairs, copyKeyPairInfo(kp))
	}

	return keyPairs
}

// copyKeyPairInfo copies key pair info without KeyMaterial.
func copyKeyPairInfo(kp *KeyPair) *KeyPair {
	return &KeyPair{
		KeyName:        kp.KeyName,
		KeyFingerprint: kp.KeyFingerprint,
		KeyPairID:      kp.KeyPairID,
		CreateTime:     kp.CreateTime,
	}
}

// findSecurityGroup finds a security group by ID or name.
func (m *MemoryStorage) findSecurityGroup(groupID, groupName string) *SecurityGroup {
	if groupID != "" {
		return m.SecurityGroups[groupID]
	}

	for _, sg := range m.SecurityGroups {
		if sg.GroupName == groupName {
			return sg
		}
	}

	return nil
}

// resolveSecurityGroups resolves security group IDs and names to GroupIdentifiers.
func (m *MemoryStorage) resolveSecurityGroups(groupIDs, groupNames []string) []GroupIdentifier {
	var result []GroupIdentifier

	for _, id := range groupIDs {
		if sg, exists := m.SecurityGroups[id]; exists {
			result = append(result, GroupIdentifier{
				GroupID:   sg.GroupID,
				GroupName: sg.GroupName,
			})
		}
	}

	for _, name := range groupNames {
		for _, sg := range m.SecurityGroups {
			if sg.GroupName == name {
				result = append(result, GroupIdentifier{
					GroupID:   sg.GroupID,
					GroupName: sg.GroupName,
				})

				break
			}
		}
	}

	return result
}

// generateID generates a random ID.
func generateID() string {
	return uuid.New().String()[:17]
}

// generatePrivateIP generates a random private IP address.
func generatePrivateIP() string {
	return fmt.Sprintf("10.0.%d.%d", randByte(), randByte())
}

// randByte returns a random byte value.
func randByte() int {
	b := make([]byte, 1)
	_, _ = rand.Read(b)

	return int(b[0])
}

// generateFingerprint generates a fingerprint from a public key.
func generateFingerprint(pubKey []byte) string {
	hash := sha256.Sum256(pubKey)

	return hex.EncodeToString(hash[:])
}

// CreateVpc creates a new VPC.
func (m *MemoryStorage) CreateVpc(_ context.Context, req *CreateVpcRequest) (*Vpc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vpc := &Vpc{
		VpcID:              "vpc-" + generateID(),
		CidrBlock:          req.CidrBlock,
		State:              "available",
		IsDefault:          false,
		InstanceTenancy:    req.InstanceTenancy,
		EnableDNSSupport:   true, // matches AWS default
		EnableDNSHostnames: false,
		Tags:               []Tag{},
	}

	if vpc.InstanceTenancy == "" {
		vpc.InstanceTenancy = "default"
	}

	m.Vpcs[vpc.VpcID] = vpc

	return vpc, nil
}

// DeleteVpc deletes a VPC.
func (m *MemoryStorage) DeleteVpc(_ context.Context, vpcID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Vpcs[vpcID]; !exists {
		return &Error{
			Code:    "InvalidVpcID.NotFound",
			Message: fmt.Sprintf("The vpc ID '%s' does not exist", vpcID),
		}
	}

	// Check for dependencies
	for _, subnet := range m.Subnets {
		if subnet.VpcID == vpcID {
			return &Error{
				Code:    "DependencyViolation",
				Message: "The vpc has dependencies and cannot be deleted",
			}
		}
	}

	for _, igw := range m.InternetGateways {
		for _, attachment := range igw.Attachments {
			if attachment.VpcID == vpcID {
				return &Error{
					Code:    "DependencyViolation",
					Message: "The vpc has dependencies and cannot be deleted",
				}
			}
		}
	}

	delete(m.Vpcs, vpcID)

	return nil
}

// DescribeVpcs describes VPCs.
func (m *MemoryStorage) DescribeVpcs(_ context.Context, vpcIDs []string) ([]*Vpc, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(vpcIDs) == 0 {
		vpcs := make([]*Vpc, 0, len(m.Vpcs))
		for _, vpc := range m.Vpcs {
			vpcs = append(vpcs, vpc)
		}

		return vpcs, nil
	}

	vpcs := make([]*Vpc, 0, len(vpcIDs))

	for _, id := range vpcIDs {
		vpc, exists := m.Vpcs[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidVpcID.NotFound",
				Message: fmt.Sprintf("The vpc ID '%s' does not exist", id),
			}
		}

		vpcs = append(vpcs, vpc)
	}

	return vpcs, nil
}

// CreateSubnet creates a new subnet.
func (m *MemoryStorage) CreateSubnet(_ context.Context, req *CreateSubnetRequest) (*Subnet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Vpcs[req.VpcID]; !exists {
		return nil, &Error{
			Code:    "InvalidVpcID.NotFound",
			Message: fmt.Sprintf("The vpc ID '%s' does not exist", req.VpcID),
		}
	}

	subnet := &Subnet{
		SubnetID:                "subnet-" + generateID(),
		VpcID:                   req.VpcID,
		CidrBlock:               req.CidrBlock,
		AvailabilityZone:        req.AvailabilityZone,
		AvailableIPAddressCount: 251, // Default available IPs for /24
		State:                   "available",
		MapPublicIPOnLaunch:     false,
		Tags:                    []Tag{},
	}

	if subnet.AvailabilityZone == "" {
		subnet.AvailabilityZone = "us-east-1a"
	}

	m.Subnets[subnet.SubnetID] = subnet

	return subnet, nil
}

// DeleteSubnet deletes a subnet.
func (m *MemoryStorage) DeleteSubnet(_ context.Context, subnetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Subnets[subnetID]; !exists {
		return &Error{
			Code:    "InvalidSubnetID.NotFound",
			Message: fmt.Sprintf("The subnet ID '%s' does not exist", subnetID),
		}
	}

	delete(m.Subnets, subnetID)

	return nil
}

// DescribeSubnets describes subnets.
func (m *MemoryStorage) DescribeSubnets(_ context.Context, subnetIDs []string, filters map[string][]string) ([]*Subnet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var subnets []*Subnet

	if len(subnetIDs) == 0 {
		for _, subnet := range m.Subnets {
			if m.matchSubnetFilters(subnet, filters) {
				subnets = append(subnets, subnet)
			}
		}

		return subnets, nil
	}

	for _, id := range subnetIDs {
		subnet, exists := m.Subnets[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidSubnetID.NotFound",
				Message: fmt.Sprintf("The subnet ID '%s' does not exist", id),
			}
		}

		if m.matchSubnetFilters(subnet, filters) {
			subnets = append(subnets, subnet)
		}
	}

	return subnets, nil
}

// matchSubnetFilters checks if a subnet matches the given filters.
func (m *MemoryStorage) matchSubnetFilters(subnet *Subnet, filters map[string][]string) bool {
	if len(filters) == 0 {
		return true
	}

	for key, values := range filters {
		switch key {
		case "vpc-id":
			if !containsString(values, subnet.VpcID) {
				return false
			}
		case "availability-zone":
			if !containsString(values, subnet.AvailabilityZone) {
				return false
			}
		}
	}

	return true
}

// CreateInternetGateway creates a new internet gateway.
func (m *MemoryStorage) CreateInternetGateway(_ context.Context, _ *CreateInternetGatewayRequest) (*InternetGateway, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	igw := &InternetGateway{
		InternetGatewayID: "igw-" + generateID(),
		Attachments:       []InternetGatewayAttachment{},
		Tags:              []Tag{},
	}

	m.InternetGateways[igw.InternetGatewayID] = igw

	return igw, nil
}

// AttachInternetGateway attaches an internet gateway to a VPC.
func (m *MemoryStorage) AttachInternetGateway(_ context.Context, igwID, vpcID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	igw, exists := m.InternetGateways[igwID]
	if !exists {
		return &Error{
			Code:    "InvalidInternetGatewayID.NotFound",
			Message: fmt.Sprintf("The internetGateway ID '%s' does not exist", igwID),
		}
	}

	if _, exists := m.Vpcs[vpcID]; !exists {
		return &Error{
			Code:    "InvalidVpcID.NotFound",
			Message: fmt.Sprintf("The vpc ID '%s' does not exist", vpcID),
		}
	}

	// Check if already attached
	for _, attachment := range igw.Attachments {
		if attachment.VpcID == vpcID {
			return &Error{
				Code:    "Resource.AlreadyAssociated",
				Message: "The internet gateway is already attached to the VPC",
			}
		}
	}

	igw.Attachments = append(igw.Attachments, InternetGatewayAttachment{
		VpcID: vpcID,
		State: "available",
	})

	return nil
}

// DescribeInternetGateways describes internet gateways.
func (m *MemoryStorage) DescribeInternetGateways(_ context.Context, igwIDs []string) ([]*InternetGateway, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(igwIDs) == 0 {
		igws := make([]*InternetGateway, 0, len(m.InternetGateways))
		for _, igw := range m.InternetGateways {
			igws = append(igws, igw)
		}

		return igws, nil
	}

	igws := make([]*InternetGateway, 0, len(igwIDs))

	for _, id := range igwIDs {
		igw, exists := m.InternetGateways[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidInternetGatewayID.NotFound",
				Message: fmt.Sprintf("The internetGateway ID '%s' does not exist", id),
			}
		}

		igws = append(igws, igw)
	}

	return igws, nil
}

// CreateRouteTable creates a new route table.
func (m *MemoryStorage) CreateRouteTable(_ context.Context, req *CreateRouteTableRequest) (*RouteTable, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Vpcs[req.VpcID]; !exists {
		return nil, &Error{
			Code:    "InvalidVpcID.NotFound",
			Message: fmt.Sprintf("The vpc ID '%s' does not exist", req.VpcID),
		}
	}

	rt := &RouteTable{
		RouteTableID: "rtb-" + generateID(),
		VpcID:        req.VpcID,
		Routes: []Route{
			{
				DestinationCidrBlock: "local",
				GatewayID:            "local",
				State:                "active",
			},
		},
		Associations: []RouteTableAssociation{},
		Tags:         []Tag{},
	}

	m.RouteTables[rt.RouteTableID] = rt

	return rt, nil
}

// CreateRoute creates a route in a route table.
func (m *MemoryStorage) CreateRoute(_ context.Context, req *CreateRouteRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rt, exists := m.RouteTables[req.RouteTableID]
	if !exists {
		return &Error{
			Code:    "InvalidRouteTableID.NotFound",
			Message: fmt.Sprintf("The routeTable ID '%s' does not exist", req.RouteTableID),
		}
	}

	// Check for duplicate route
	for _, route := range rt.Routes {
		if route.DestinationCidrBlock == req.DestinationCidrBlock {
			return &Error{
				Code:    "RouteAlreadyExists",
				Message: "The route already exists",
			}
		}
	}

	route := Route{
		DestinationCidrBlock: req.DestinationCidrBlock,
		GatewayID:            req.GatewayID,
		NatGatewayID:         req.NatGatewayID,
		State:                "active",
	}

	rt.Routes = append(rt.Routes, route)

	return nil
}

// AssociateRouteTable associates a route table with a subnet.
func (m *MemoryStorage) AssociateRouteTable(_ context.Context, req *AssociateRouteTableRequest) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rt, exists := m.RouteTables[req.RouteTableID]
	if !exists {
		return "", &Error{
			Code:    "InvalidRouteTableID.NotFound",
			Message: fmt.Sprintf("The routeTable ID '%s' does not exist", req.RouteTableID),
		}
	}

	if _, exists := m.Subnets[req.SubnetID]; !exists {
		return "", &Error{
			Code:    "InvalidSubnetID.NotFound",
			Message: fmt.Sprintf("The subnet ID '%s' does not exist", req.SubnetID),
		}
	}

	associationID := "rtbassoc-" + generateID()
	rt.Associations = append(rt.Associations, RouteTableAssociation{
		RouteTableAssociationID: associationID,
		RouteTableID:            req.RouteTableID,
		SubnetID:                req.SubnetID,
		Main:                    false,
	})

	return associationID, nil
}

// DescribeRouteTables describes route tables.
func (m *MemoryStorage) DescribeRouteTables(_ context.Context, rtbIDs []string) ([]*RouteTable, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(rtbIDs) == 0 {
		rts := make([]*RouteTable, 0, len(m.RouteTables))
		for _, rt := range m.RouteTables {
			rts = append(rts, rt)
		}

		return rts, nil
	}

	rts := make([]*RouteTable, 0, len(rtbIDs))

	for _, id := range rtbIDs {
		rt, exists := m.RouteTables[id]
		if !exists {
			return nil, &Error{
				Code:    "InvalidRouteTableID.NotFound",
				Message: fmt.Sprintf("The routeTable ID '%s' does not exist", id),
			}
		}

		rts = append(rts, rt)
	}

	return rts, nil
}

// CreateNatGateway creates a new NAT gateway.
func (m *MemoryStorage) CreateNatGateway(_ context.Context, req *CreateNatGatewayRequest) (*NatGateway, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subnet, exists := m.Subnets[req.SubnetID]
	if !exists {
		return nil, &Error{
			Code:    "InvalidSubnetID.NotFound",
			Message: fmt.Sprintf("The subnet ID '%s' does not exist", req.SubnetID),
		}
	}

	natgw := &NatGateway{
		NatGatewayID: "nat-" + generateID(),
		SubnetID:     req.SubnetID,
		VpcID:        subnet.VpcID,
		State:        "available",
		Tags:         []Tag{},
	}

	if req.ConnectivityType == "private" {
		natgw.ConnectivityType = "private"
	} else {
		natgw.ConnectivityType = "public"
		natgw.AllocationID = req.AllocationID
	}

	m.NatGateways[natgw.NatGatewayID] = natgw

	return natgw, nil
}

// DescribeNatGateways describes NAT gateways.
func (m *MemoryStorage) DescribeNatGateways(_ context.Context, natgwIDs []string) ([]*NatGateway, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(natgwIDs) == 0 {
		natgws := make([]*NatGateway, 0, len(m.NatGateways))
		for _, natgw := range m.NatGateways {
			natgws = append(natgws, natgw)
		}

		return natgws, nil
	}

	natgws := make([]*NatGateway, 0, len(natgwIDs))

	for _, id := range natgwIDs {
		natgw, exists := m.NatGateways[id]
		if !exists {
			return nil, &Error{
				Code:    "NatGatewayNotFound",
				Message: fmt.Sprintf("The natGateway ID '%s' does not exist", id),
			}
		}

		natgws = append(natgws, natgw)
	}

	return natgws, nil
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}

	return false
}

// CreateTags adds or overwrites tags on the listed resources.
func (m *MemoryStorage) CreateTags(_ context.Context, resourceIDs []string, tags []Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range resourceIDs {
		current, _, err := m.lookupTagsLocked(id)
		if err != nil {
			return err
		}

		updated := mergeTags(current, tags)

		if err := m.setTagsLocked(id, updated); err != nil {
			return err
		}
	}

	return nil
}

// ModifyVpcAttribute applies VPC attribute updates. Only set (non-nil) fields
// are touched, matching the AWS one-attribute-per-call semantics.
func (m *MemoryStorage) ModifyVpcAttribute(_ context.Context, vpcID string, updates VpcAttributeUpdates) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vpc, ok := m.Vpcs[vpcID]
	if !ok {
		return &Error{
			Code:    "InvalidVpcID.NotFound",
			Message: fmt.Sprintf("The vpc ID '%s' does not exist", vpcID),
		}
	}

	if updates.EnableDNSHostnames != nil {
		vpc.EnableDNSHostnames = *updates.EnableDNSHostnames
	}

	if updates.EnableDNSSupport != nil {
		vpc.EnableDNSSupport = *updates.EnableDNSSupport
	}

	return nil
}

// ModifySubnetAttribute applies subnet attribute updates.
func (m *MemoryStorage) ModifySubnetAttribute(_ context.Context, subnetID string, updates SubnetAttributeUpdates) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subnet, ok := m.Subnets[subnetID]
	if !ok {
		return &Error{
			Code:    "InvalidSubnetID.NotFound",
			Message: fmt.Sprintf("The subnet ID '%s' does not exist", subnetID),
		}
	}

	if updates.MapPublicIPOnLaunch != nil {
		subnet.MapPublicIPOnLaunch = *updates.MapPublicIPOnLaunch
	}

	_ = updates.AssignIPv6AddressOnCreation // accepted but not modeled

	return nil
}

// DeleteTags removes the listed tags from the resources. An empty tag list
// removes all tags. A tag with an empty Value matches any value.
func (m *MemoryStorage) DeleteTags(_ context.Context, resourceIDs []string, tags []Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range resourceIDs {
		current, _, err := m.lookupTagsLocked(id)
		if err != nil {
			return err
		}

		var updated []Tag
		if len(tags) == 0 {
			updated = nil
		} else {
			updated = removeTags(current, tags)
		}

		if err := m.setTagsLocked(id, updated); err != nil {
			return err
		}
	}

	return nil
}

// DescribeTags returns all tags across all resources, optionally filtered by
// resource-id or key. Other filters are accepted for compatibility but ignored.
func (m *MemoryStorage) DescribeTags(_ context.Context, filters map[string][]string) ([]TagDescription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resourceIDs := filters["resource-id"]
	keys := filters["key"]

	descriptions := make([]TagDescription, 0)

	for id, getTags := range m.tagSourcesLocked() {
		if len(resourceIDs) > 0 && !containsString(resourceIDs, id) {
			continue
		}

		resourceType := resourceTypeForID(id)

		for _, t := range getTags() {
			if len(keys) > 0 && !containsString(keys, t.Key) {
				continue
			}

			descriptions = append(descriptions, TagDescription{
				ResourceID:   id,
				ResourceType: resourceType,
				Key:          t.Key,
				Value:        t.Value,
			})
		}
	}

	sort.Slice(descriptions, func(i, j int) bool {
		if descriptions[i].ResourceID != descriptions[j].ResourceID {
			return descriptions[i].ResourceID < descriptions[j].ResourceID
		}

		return descriptions[i].Key < descriptions[j].Key
	})

	return descriptions, nil
}

// lookupTagsLocked returns the current tags and a setter for the resource ID.
// The caller must hold the appropriate lock.
func (m *MemoryStorage) lookupTagsLocked(id string) ([]Tag, func([]Tag), error) {
	if vpc, ok := m.Vpcs[id]; ok {
		return vpc.Tags, func(t []Tag) { vpc.Tags = t }, nil
	}

	if subnet, ok := m.Subnets[id]; ok {
		return subnet.Tags, func(t []Tag) { subnet.Tags = t }, nil
	}

	if igw, ok := m.InternetGateways[id]; ok {
		return igw.Tags, func(t []Tag) { igw.Tags = t }, nil
	}

	if rt, ok := m.RouteTables[id]; ok {
		return rt.Tags, func(t []Tag) { rt.Tags = t }, nil
	}

	if natgw, ok := m.NatGateways[id]; ok {
		return natgw.Tags, func(t []Tag) { natgw.Tags = t }, nil
	}

	if inst, ok := m.Instances[id]; ok {
		return inst.Tags, func(t []Tag) { inst.Tags = t }, nil
	}

	if sg, ok := m.SecurityGroups[id]; ok {
		return sg.Tags, func(t []Tag) { sg.Tags = t }, nil
	}

	if kp, ok := m.KeyPairs[id]; ok {
		return kp.Tags, func(t []Tag) { kp.Tags = t }, nil
	}

	return nil, nil, &Error{
		Code:    "InvalidID",
		Message: fmt.Sprintf("The ID '%s' is not valid", id),
	}
}

func (m *MemoryStorage) setTagsLocked(id string, tags []Tag) error {
	_, set, err := m.lookupTagsLocked(id)
	if err != nil {
		return err
	}

	set(tags)

	return nil
}

// tagSourcesLocked returns a map of resource-id to a closure returning that
// resource's tag list. Used to iterate every taggable resource.
func (m *MemoryStorage) tagSourcesLocked() map[string]func() []Tag {
	out := make(map[string]func() []Tag)

	for id, v := range m.Vpcs {
		out[id] = func() []Tag { return v.Tags }
	}

	for id, v := range m.Subnets {
		out[id] = func() []Tag { return v.Tags }
	}

	for id, v := range m.InternetGateways {
		out[id] = func() []Tag { return v.Tags }
	}

	for id, v := range m.RouteTables {
		out[id] = func() []Tag { return v.Tags }
	}

	for id, v := range m.NatGateways {
		out[id] = func() []Tag { return v.Tags }
	}

	for id, v := range m.Instances {
		out[id] = func() []Tag { return v.Tags }
	}

	for id, v := range m.SecurityGroups {
		out[id] = func() []Tag { return v.Tags }
	}

	for id, v := range m.KeyPairs {
		out[id] = func() []Tag { return v.Tags }
	}

	return out
}

// resourceTypePrefixes maps EC2 resource ID prefixes to their resource-type
// strings. Order does not matter; longer-prefix matches are not required since
// EC2 prefixes do not collide.
var resourceTypePrefixes = map[string]string{
	"vpc-":    "vpc",
	"subnet-": "subnet",
	"igw-":    "internet-gateway",
	"rtb-":    "route-table",
	"nat-":    "natgateway",
	"i-":      "instance",
	"sg-":     "security-group",
	"key-":    "key-pair",
}

// resourceTypeForID maps an EC2 resource ID prefix to its resource-type string.
func resourceTypeForID(id string) string {
	for prefix, rtype := range resourceTypePrefixes {
		if strings.HasPrefix(id, prefix) {
			return rtype
		}
	}

	return ""
}

// mergeTags upserts new tags into existing, replacing values for matching keys.
func mergeTags(existing, incoming []Tag) []Tag {
	merged := make([]Tag, 0, len(existing)+len(incoming))
	merged = append(merged, existing...)

	for _, t := range incoming {
		replaced := false

		for i, e := range merged {
			if e.Key == t.Key {
				merged[i].Value = t.Value
				replaced = true

				break
			}
		}

		if !replaced {
			merged = append(merged, t)
		}
	}

	return merged
}

// removeTags returns existing minus any matches in toRemove. A toRemove entry
// with an empty Value matches all values for that key.
func removeTags(existing, toRemove []Tag) []Tag {
	out := make([]Tag, 0, len(existing))

	for _, e := range existing {
		drop := false

		for _, r := range toRemove {
			if r.Key != e.Key {
				continue
			}

			if r.Value == "" || r.Value == e.Value {
				drop = true

				break
			}
		}

		if !drop {
			out = append(out, e)
		}
	}

	return out
}
