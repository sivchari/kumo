package gamelift

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

// Error codes.
const (
	errNotFoundException        = "NotFoundException"
	errInvalidRequestException  = "InvalidRequestException"
	errConflictException        = "ConflictException"
	errInternalServiceException = "InternalServiceException"
	errLimitExceededException   = "LimitExceededException"
)

// Default values.
const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "123456789012"
	defaultIPAddress = "10.0.0.1"
	defaultPort      = int32(7777)
)

// Build status values.
const (
	buildStatusInitialized = "INITIALIZED"
	buildStatusReady       = "READY"
)

// Fleet status values.
const (
	fleetStatusNew    = "NEW"
	fleetStatusActive = "ACTIVE"
)

// Game session status values.
const (
	gameSessionStatusActive     = "ACTIVE"
	gameSessionStatusActivating = "ACTIVATING"
)

// Player session status values.
const (
	playerSessionStatusReserved = "RESERVED"
	playerSessionStatusActive   = "ACTIVE"
)

// Storage defines the GameLift service storage interface.
type Storage interface {
	// Build operations
	CreateBuild(ctx context.Context, req *CreateBuildRequest) (*Build, error)
	DescribeBuild(ctx context.Context, buildID string) (*Build, error)
	ListBuilds(ctx context.Context, status string, limit int32) ([]*Build, error)
	DeleteBuild(ctx context.Context, buildID string) error

	// Fleet operations
	CreateFleet(ctx context.Context, req *CreateFleetRequest) (*Fleet, error)
	DescribeFleetAttributes(ctx context.Context, fleetIDs []string) ([]*Fleet, error)
	ListFleets(ctx context.Context, buildID string, limit int32) ([]string, error)
	DeleteFleet(ctx context.Context, fleetID string) error

	// Game session operations
	CreateGameSession(ctx context.Context, req *CreateGameSessionRequest) (*GameSession, error)
	DescribeGameSessions(ctx context.Context, fleetID, gameSessionID string) ([]*GameSession, error)
	UpdateGameSession(ctx context.Context, req *UpdateGameSessionRequest) (*GameSession, error)

	// Player session operations
	CreatePlayerSession(ctx context.Context, gameSessionID, playerID string) (*PlayerSession, error)
	CreatePlayerSessions(ctx context.Context, gameSessionID string, playerIDs []string) ([]*PlayerSession, error)
	DescribePlayerSessions(ctx context.Context, gameSessionID, playerSessionID, playerID string) ([]*PlayerSession, error)
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
	mu             sync.RWMutex              `json:"-"`
	Builds         map[string]*Build         `json:"builds"`
	Fleets         map[string]*Fleet         `json:"fleets"`
	GameSessions   map[string]*GameSession   `json:"gameSessions"`
	PlayerSessions map[string]*PlayerSession `json:"playerSessions"`
	region         string
	accountID      string
	dataDir        string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Builds:         make(map[string]*Build),
		Fleets:         make(map[string]*Fleet),
		GameSessions:   make(map[string]*GameSession),
		PlayerSessions: make(map[string]*PlayerSession),
		region:         region,
		accountID:      defaultAccountID,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "gamelift", s)
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

	if m.Builds == nil {
		m.Builds = make(map[string]*Build)
	}

	if m.Fleets == nil {
		m.Fleets = make(map[string]*Fleet)
	}

	if m.GameSessions == nil {
		m.GameSessions = make(map[string]*GameSession)
	}

	if m.PlayerSessions == nil {
		m.PlayerSessions = make(map[string]*PlayerSession)
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

	_ = storage.SaveBytes(m.dataDir, "gamelift", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "gamelift", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateBuild creates a new build.
func (m *MemoryStorage) CreateBuild(_ context.Context, req *CreateBuildRequest) (*Build, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	buildID := "build-" + uuid.New().String()[:8]
	buildARN := generateBuildARN(m.region, m.accountID, buildID)

	build := &Build{
		BuildID:         buildID,
		BuildARN:        buildARN,
		Name:            req.Name,
		Version:         req.Version,
		Status:          buildStatusInitialized,
		SizeOnDisk:      0,
		OperatingSystem: defaultString(req.OperatingSystem, "AMAZON_LINUX_2"),
		CreationTime:    time.Now(),
	}

	m.Builds[buildID] = build

	m.saveLocked()

	return build, nil
}

// DescribeBuild describes a build.
func (m *MemoryStorage) DescribeBuild(_ context.Context, buildID string) (*Build, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	build, exists := m.Builds[buildID]
	if !exists {
		return nil, &Error{Code: errNotFoundException, Message: "Build not found: " + buildID}
	}

	return build, nil
}

// ListBuilds lists builds.
func (m *MemoryStorage) ListBuilds(_ context.Context, status string, limit int32) ([]*Build, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Build, 0, len(m.Builds))

	for _, build := range m.Builds {
		if status != "" && build.Status != status {
			continue
		}

		result = append(result, build)

		//nolint:gosec // len(result) is bounded by the number of builds which is limited.
		if limit > 0 && int32(len(result)) >= limit {
			break
		}
	}

	return result, nil
}

// DeleteBuild deletes a build.
func (m *MemoryStorage) DeleteBuild(_ context.Context, buildID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Builds[buildID]; !exists {
		return &Error{Code: errNotFoundException, Message: "Build not found: " + buildID}
	}

	delete(m.Builds, buildID)

	m.saveLocked()

	return nil
}

// CreateFleet creates a new fleet.
func (m *MemoryStorage) CreateFleet(_ context.Context, req *CreateFleetRequest) (*Fleet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, &Error{Code: errInvalidRequestException, Message: "Name is required"}
	}

	fleetID := "fleet-" + uuid.New().String()[:8]
	fleetARN := generateFleetARN(m.region, m.accountID, fleetID)

	fleet := &Fleet{
		FleetID:                        fleetID,
		FleetARN:                       fleetARN,
		Name:                           req.Name,
		Description:                    req.Description,
		BuildID:                        req.BuildID,
		Status:                         fleetStatusActive,
		FleetType:                      defaultString(req.FleetType, "ON_DEMAND"),
		InstanceType:                   defaultString(req.EC2InstanceType, "c5.large"),
		ServerLaunchPath:               req.ServerLaunchPath,
		NewGameSessionProtectionPolicy: defaultString(req.NewGameSessionProtectionPolicy, "NoProtection"),
		CreationTime:                   time.Now(),
	}

	m.Fleets[fleetID] = fleet

	m.saveLocked()

	return fleet, nil
}

// DescribeFleetAttributes describes fleet attributes.
func (m *MemoryStorage) DescribeFleetAttributes(_ context.Context, fleetIDs []string) ([]*Fleet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(fleetIDs) == 0 {
		// Return all fleets
		result := make([]*Fleet, 0, len(m.Fleets))
		for _, fleet := range m.Fleets {
			result = append(result, fleet)
		}

		return result, nil
	}

	// Return specified fleets
	result := make([]*Fleet, 0, len(fleetIDs))

	for _, fleetID := range fleetIDs {
		if fleet, exists := m.Fleets[fleetID]; exists {
			result = append(result, fleet)
		}
	}

	return result, nil
}

// ListFleets lists fleet IDs.
func (m *MemoryStorage) ListFleets(_ context.Context, buildID string, limit int32) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.Fleets))

	for _, fleet := range m.Fleets {
		if buildID != "" && fleet.BuildID != buildID {
			continue
		}

		result = append(result, fleet.FleetID)

		//nolint:gosec // len(result) is bounded by the number of fleets which is limited.
		if limit > 0 && int32(len(result)) >= limit {
			break
		}
	}

	return result, nil
}

// DeleteFleet deletes a fleet.
func (m *MemoryStorage) DeleteFleet(_ context.Context, fleetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Fleets[fleetID]; !exists {
		return &Error{Code: errNotFoundException, Message: "Fleet not found: " + fleetID}
	}

	delete(m.Fleets, fleetID)

	m.saveLocked()

	return nil
}

// CreateGameSession creates a new game session.
func (m *MemoryStorage) CreateGameSession(_ context.Context, req *CreateGameSessionRequest) (*GameSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.FleetID == "" {
		return nil, &Error{Code: errInvalidRequestException, Message: "FleetId is required"}
	}

	fleet, exists := m.Fleets[req.FleetID]
	if !exists {
		return nil, &Error{Code: errNotFoundException, Message: "Fleet not found: " + req.FleetID}
	}

	gameSessionID := "gsess-" + uuid.New().String()[:8]
	gameSessionARN := generateGameSessionARN(m.region, m.accountID, fleet.FleetID, gameSessionID)

	gameSession := &GameSession{
		GameSessionID:             gameSessionID,
		GameSessionARN:            gameSessionARN,
		FleetID:                   fleet.FleetID,
		FleetARN:                  fleet.FleetARN,
		Name:                      req.Name,
		Status:                    gameSessionStatusActive,
		CurrentPlayerSessionCount: 0,
		MaximumPlayerSessionCount: req.MaximumPlayerSessionCount,
		IPAddress:                 defaultIPAddress,
		Port:                      defaultPort,
		CreationTime:              time.Now(),
	}

	m.GameSessions[gameSessionID] = gameSession

	m.saveLocked()

	return gameSession, nil
}

// DescribeGameSessions describes game sessions.
func (m *MemoryStorage) DescribeGameSessions(_ context.Context, fleetID, gameSessionID string) ([]*GameSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if gameSessionID != "" {
		session, exists := m.GameSessions[gameSessionID]
		if !exists {
			return []*GameSession{}, nil
		}

		return []*GameSession{session}, nil
	}

	result := make([]*GameSession, 0, len(m.GameSessions))

	for _, session := range m.GameSessions {
		if fleetID != "" && session.FleetID != fleetID {
			continue
		}

		result = append(result, session)
	}

	return result, nil
}

// UpdateGameSession updates a game session.
func (m *MemoryStorage) UpdateGameSession(_ context.Context, req *UpdateGameSessionRequest) (*GameSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.GameSessions[req.GameSessionID]
	if !exists {
		return nil, &Error{Code: errNotFoundException, Message: "Game session not found: " + req.GameSessionID}
	}

	if req.MaximumPlayerSessionCount != nil {
		session.MaximumPlayerSessionCount = *req.MaximumPlayerSessionCount
	}

	if req.Name != "" {
		session.Name = req.Name
	}

	m.saveLocked()

	return session, nil
}

// CreatePlayerSession creates a new player session.
func (m *MemoryStorage) CreatePlayerSession(_ context.Context, gameSessionID, playerID string) (*PlayerSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	gameSession, exists := m.GameSessions[gameSessionID]
	if !exists {
		return nil, &Error{Code: errNotFoundException, Message: "Game session not found: " + gameSessionID}
	}

	if gameSession.CurrentPlayerSessionCount >= gameSession.MaximumPlayerSessionCount {
		return nil, &Error{Code: errInvalidRequestException, Message: "Game session is full"}
	}

	playerSessionID := "psess-" + uuid.New().String()[:8]

	playerSession := &PlayerSession{
		PlayerSessionID: playerSessionID,
		GameSessionID:   gameSessionID,
		FleetID:         gameSession.FleetID,
		FleetARN:        gameSession.FleetARN,
		PlayerID:        playerID,
		Status:          playerSessionStatusReserved,
		IPAddress:       gameSession.IPAddress,
		Port:            gameSession.Port,
		CreationTime:    time.Now(),
	}

	m.PlayerSessions[playerSessionID] = playerSession
	gameSession.CurrentPlayerSessionCount++

	m.saveLocked()

	return playerSession, nil
}

// CreatePlayerSessions creates multiple player sessions.
func (m *MemoryStorage) CreatePlayerSessions(_ context.Context, gameSessionID string, playerIDs []string) ([]*PlayerSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	gameSession, exists := m.GameSessions[gameSessionID]
	if !exists {
		return nil, &Error{Code: errNotFoundException, Message: "Game session not found: " + gameSessionID}
	}

	//nolint:gosec // len(playerIDs) is bounded by the request, which is limited by AWS SDK.
	if gameSession.CurrentPlayerSessionCount+int32(len(playerIDs)) > gameSession.MaximumPlayerSessionCount {
		return nil, &Error{Code: errInvalidRequestException, Message: "Not enough capacity for all players"}
	}

	result := make([]*PlayerSession, 0, len(playerIDs))

	for _, playerID := range playerIDs {
		playerSessionID := "psess-" + uuid.New().String()[:8]

		playerSession := &PlayerSession{
			PlayerSessionID: playerSessionID,
			GameSessionID:   gameSessionID,
			FleetID:         gameSession.FleetID,
			FleetARN:        gameSession.FleetARN,
			PlayerID:        playerID,
			Status:          playerSessionStatusReserved,
			IPAddress:       gameSession.IPAddress,
			Port:            gameSession.Port,
			CreationTime:    time.Now(),
		}

		m.PlayerSessions[playerSessionID] = playerSession
		result = append(result, playerSession)
	}

	//nolint:gosec // len(playerIDs) is bounded by the request, which is limited by AWS SDK.
	gameSession.CurrentPlayerSessionCount += int32(len(playerIDs))

	m.saveLocked()

	return result, nil
}

// DescribePlayerSessions describes player sessions.
func (m *MemoryStorage) DescribePlayerSessions(_ context.Context, gameSessionID, playerSessionID, playerID string) ([]*PlayerSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if playerSessionID != "" {
		session, exists := m.PlayerSessions[playerSessionID]
		if !exists {
			return []*PlayerSession{}, nil
		}

		return []*PlayerSession{session}, nil
	}

	result := make([]*PlayerSession, 0, len(m.PlayerSessions))

	for _, session := range m.PlayerSessions {
		if gameSessionID != "" && session.GameSessionID != gameSessionID {
			continue
		}

		if playerID != "" && session.PlayerID != playerID {
			continue
		}

		result = append(result, session)
	}

	return result, nil
}

// Helper functions.

func generateBuildARN(region, accountID, buildID string) string {
	return fmt.Sprintf("arn:aws:gamelift:%s:%s:build/%s", region, accountID, buildID)
}

func generateFleetARN(region, accountID, fleetID string) string {
	return fmt.Sprintf("arn:aws:gamelift:%s:%s:fleet/%s", region, accountID, fleetID)
}

func generateGameSessionARN(region, accountID, fleetID, gameSessionID string) string {
	return fmt.Sprintf("arn:aws:gamelift:%s:%s:gamesession/%s/%s", region, accountID, fleetID, gameSessionID)
}

func defaultString(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}

	return value
}
