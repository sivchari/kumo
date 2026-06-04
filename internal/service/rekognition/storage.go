package rekognition

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion                 = "us-east-1"
	defaultAccountID              = "123456789012"
	defaultFaceModelVersion       = "6.0"
	defaultLabelModelVersion      = "3.0"
	defaultTextModelVersion       = "3.0"
	defaultModerationModelVersion = "6.0"
)

// Collection represents a Rekognition collection.
type Collection struct {
	CollectionID      string
	CollectionArn     string
	FaceModelVersion  string
	CreationTimestamp float64
	FaceCount         int64
	UserCount         int64
	Tags              map[string]string
	Faces             map[string]*Face
}

// Storage defines the interface for Rekognition storage.
type Storage interface {
	// Collection management
	CreateCollection(ctx context.Context, req *CreateCollectionRequest) (*CreateCollectionResponse, error)
	DeleteCollection(ctx context.Context, collectionID string) (*DeleteCollectionResponse, error)
	ListCollections(ctx context.Context, req *ListCollectionsRequest) (*ListCollectionsResponse, error)
	DescribeCollection(ctx context.Context, collectionID string) (*DescribeCollectionResponse, error)

	// Face operations
	IndexFaces(ctx context.Context, req *IndexFacesRequest) (*IndexFacesResponse, error)
	ListFaces(ctx context.Context, req *ListFacesRequest) (*ListFacesResponse, error)
	SearchFaces(ctx context.Context, req *SearchFacesRequest) (*SearchFacesResponse, error)
	DeleteFaces(ctx context.Context, req *DeleteFacesRequest) (*DeleteFacesResponse, error)

	// Detection operations (stateless - return mock data)
	DetectFaces(ctx context.Context, req *DetectFacesRequest) (*DetectFacesResponse, error)
	DetectLabels(ctx context.Context, req *DetectLabelsRequest) (*DetectLabelsResponse, error)
	DetectText(ctx context.Context, req *DetectTextRequest) (*DetectTextResponse, error)
	RecognizeCelebrities(ctx context.Context, req *RecognizeCelebritiesRequest) (*RecognizeCelebritiesResponse, error)
	DetectModerationLabels(ctx context.Context, req *DetectModerationLabelsRequest) (*DetectModerationLabelsResponse, error)
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

// MemoryStorage implements in-memory storage for Rekognition.
type MemoryStorage struct {
	mu          sync.RWMutex           `json:"-"`
	Collections map[string]*Collection `json:"collections"`
	dataDir     string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Collections: make(map[string]*Collection),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "rekognition", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Collections == nil {
		s.Collections = make(map[string]*Collection)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "rekognition", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "rekognition", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateCollection creates a new collection.
func (s *MemoryStorage) CreateCollection(_ context.Context, req *CreateCollectionRequest) (*CreateCollectionResponse, error) {
	if req.CollectionID == "" {
		return nil, &ServiceError{
			Code:    errInvalidParameter,
			Message: "CollectionId is required",
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Collections[req.CollectionID]; exists {
		return nil, &ServiceError{
			Code:    errResourceExists,
			Message: fmt.Sprintf("Collection with id %s already exists", req.CollectionID),
		}
	}

	arn := fmt.Sprintf("arn:aws:rekognition:%s:%s:collection/%s", defaultRegion, defaultAccountID, req.CollectionID)

	collection := &Collection{
		CollectionID:      req.CollectionID,
		CollectionArn:     arn,
		FaceModelVersion:  defaultFaceModelVersion,
		CreationTimestamp: float64(time.Now().Unix()),
		FaceCount:         0,
		UserCount:         0,
		Tags:              req.Tags,
		Faces:             make(map[string]*Face),
	}

	s.Collections[req.CollectionID] = collection

	s.saveLocked()

	return &CreateCollectionResponse{
		CollectionArn:    arn,
		FaceModelVersion: defaultFaceModelVersion,
		StatusCode:       200,
	}, nil
}

// DeleteCollection deletes a collection.
func (s *MemoryStorage) DeleteCollection(_ context.Context, collectionID string) (*DeleteCollectionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Collections[collectionID]; !exists {
		return nil, &ServiceError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Collection with id %s not found", collectionID),
		}
	}

	delete(s.Collections, collectionID)

	s.saveLocked()

	return &DeleteCollectionResponse{
		StatusCode: 200,
	}, nil
}

// ListCollections lists all collections.
func (s *MemoryStorage) ListCollections(_ context.Context, _ *ListCollectionsRequest) (*ListCollectionsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	collectionIDs := make([]string, 0, len(s.Collections))
	faceModelVersions := make([]string, 0, len(s.Collections))

	for _, c := range s.Collections {
		collectionIDs = append(collectionIDs, c.CollectionID)
		faceModelVersions = append(faceModelVersions, c.FaceModelVersion)
	}

	return &ListCollectionsResponse{
		CollectionIDs:     collectionIDs,
		FaceModelVersions: faceModelVersions,
	}, nil
}

// DescribeCollection describes a collection.
func (s *MemoryStorage) DescribeCollection(_ context.Context, collectionID string) (*DescribeCollectionResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	collection, exists := s.Collections[collectionID]
	if !exists {
		return nil, &ServiceError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Collection with id %s not found", collectionID),
		}
	}

	return &DescribeCollectionResponse{
		CollectionARN:     collection.CollectionArn,
		CreationTimestamp: collection.CreationTimestamp,
		FaceCount:         collection.FaceCount,
		FaceModelVersion:  collection.FaceModelVersion,
		UserCount:         collection.UserCount,
	}, nil
}

// IndexFaces indexes faces in a collection.
func (s *MemoryStorage) IndexFaces(_ context.Context, req *IndexFacesRequest) (*IndexFacesResponse, error) {
	if req.CollectionID == "" {
		return nil, &ServiceError{
			Code:    errInvalidParameter,
			Message: "CollectionId is required",
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	collection, exists := s.Collections[req.CollectionID]
	if !exists {
		return nil, &ServiceError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Collection with id %s not found", req.CollectionID),
		}
	}

	// Generate mock face data
	faceID := uuid.New().String()
	imageID := uuid.New().String()

	face := &Face{
		FaceID:                 faceID,
		ImageID:                imageID,
		ExternalImageID:        req.ExternalImageID,
		Confidence:             99.99,
		IndexFacesModelVersion: defaultFaceModelVersion,
		BoundingBox: &BoundingBox{
			Height: 0.3,
			Left:   0.2,
			Top:    0.1,
			Width:  0.25,
		},
	}

	collection.Faces[faceID] = face
	collection.FaceCount++

	s.saveLocked()

	faceDetail := &FaceDetail{
		BoundingBox: face.BoundingBox,
		Confidence:  face.Confidence,
		Landmarks:   generateMockLandmarks(),
		Pose:        &Pose{Pitch: 0.5, Roll: 1.2, Yaw: -0.3},
		Quality:     &ImageQuality{Brightness: 80.0, Sharpness: 95.0},
	}

	return &IndexFacesResponse{
		FaceModelVersion: defaultFaceModelVersion,
		FaceRecords: []FaceRecord{
			{
				Face:       face,
				FaceDetail: faceDetail,
			},
		},
		UnindexedFaces: []UnindexedFace{},
	}, nil
}

// ListFaces lists faces in a collection.
func (s *MemoryStorage) ListFaces(_ context.Context, req *ListFacesRequest) (*ListFacesResponse, error) {
	if req.CollectionID == "" {
		return nil, &ServiceError{
			Code:    errInvalidParameter,
			Message: "CollectionId is required",
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	collection, exists := s.Collections[req.CollectionID]
	if !exists {
		return nil, &ServiceError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Collection with id %s not found", req.CollectionID),
		}
	}

	faces := make([]Face, 0, len(collection.Faces))

	for _, f := range collection.Faces {
		faces = append(faces, *f)
	}

	return &ListFacesResponse{
		FaceModelVersion: defaultFaceModelVersion,
		Faces:            faces,
	}, nil
}

// SearchFaces searches for faces in a collection.
func (s *MemoryStorage) SearchFaces(_ context.Context, req *SearchFacesRequest) (*SearchFacesResponse, error) {
	if req.CollectionID == "" {
		return nil, &ServiceError{
			Code:    errInvalidParameter,
			Message: "CollectionId is required",
		}
	}

	if req.FaceID == "" {
		return nil, &ServiceError{
			Code:    errInvalidParameter,
			Message: "FaceId is required",
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	collection, exists := s.Collections[req.CollectionID]
	if !exists {
		return nil, &ServiceError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Collection with id %s not found", req.CollectionID),
		}
	}

	if _, faceExists := collection.Faces[req.FaceID]; !faceExists {
		return nil, &ServiceError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Face with id %s not found in collection", req.FaceID),
		}
	}

	// Return mock matches (excluding the searched face)
	matches := make([]FaceMatch, 0)

	for faceID, face := range collection.Faces {
		if faceID != req.FaceID {
			matches = append(matches, FaceMatch{
				Face:       face,
				Similarity: 95.5,
			})
		}
	}

	return &SearchFacesResponse{
		FaceModelVersion: defaultFaceModelVersion,
		FaceMatches:      matches,
		SearchedFaceID:   req.FaceID,
	}, nil
}

// DeleteFaces deletes faces from a collection.
func (s *MemoryStorage) DeleteFaces(_ context.Context, req *DeleteFacesRequest) (*DeleteFacesResponse, error) {
	if req.CollectionID == "" {
		return nil, &ServiceError{
			Code:    errInvalidParameter,
			Message: "CollectionId is required",
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	collection, exists := s.Collections[req.CollectionID]
	if !exists {
		return nil, &ServiceError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Collection with id %s not found", req.CollectionID),
		}
	}

	deletedFaces := make([]string, 0)

	for _, faceID := range req.FaceIDs {
		if _, faceExists := collection.Faces[faceID]; faceExists {
			delete(collection.Faces, faceID)

			collection.FaceCount--

			deletedFaces = append(deletedFaces, faceID)
		}
	}

	s.saveLocked()

	return &DeleteFacesResponse{
		DeletedFaces: deletedFaces,
	}, nil
}

// DetectFaces detects faces in an image.
func (s *MemoryStorage) DetectFaces(_ context.Context, _ *DetectFacesRequest) (*DetectFacesResponse, error) {
	// Return mock face detection results
	return &DetectFacesResponse{
		FaceDetails: []FaceDetail{
			{
				BoundingBox: &BoundingBox{
					Height: 0.35,
					Left:   0.25,
					Top:    0.15,
					Width:  0.28,
				},
				Confidence: 99.95,
				AgeRange:   &AgeRange{Low: 25, High: 35},
				Smile:      &Attribute{Confidence: 98.5, Value: true},
				Eyeglasses: &Attribute{Confidence: 99.2, Value: false},
				Sunglasses: &Attribute{Confidence: 99.8, Value: false},
				Gender:     &Gender{Confidence: 99.5, Value: "Female"},
				Beard:      &Attribute{Confidence: 99.9, Value: false},
				Mustache:   &Attribute{Confidence: 99.9, Value: false},
				EyesOpen:   &Attribute{Confidence: 99.3, Value: true},
				MouthOpen:  &Attribute{Confidence: 98.7, Value: false},
				Emotions: []Emotion{
					{Type: "HAPPY", Confidence: 95.5},
					{Type: "CALM", Confidence: 4.2},
					{Type: "SURPRISED", Confidence: 0.3},
				},
				Landmarks: generateMockLandmarks(),
				Pose:      &Pose{Pitch: 2.5, Roll: -1.3, Yaw: 4.2},
				Quality:   &ImageQuality{Brightness: 82.5, Sharpness: 91.3},
			},
		},
	}, nil
}

// DetectLabels detects labels in an image.
func (s *MemoryStorage) DetectLabels(_ context.Context, _ *DetectLabelsRequest) (*DetectLabelsResponse, error) {
	// Return mock label detection results
	return &DetectLabelsResponse{
		LabelModelVersion: defaultLabelModelVersion,
		Labels: []Label{
			{
				Name:       "Person",
				Confidence: 99.5,
				Parents:    []Parent{},
				Categories: []LabelCategory{{Name: "Person Description"}},
				Instances: []Instance{
					{
						BoundingBox: &BoundingBox{Height: 0.8, Left: 0.1, Top: 0.1, Width: 0.4},
						Confidence:  98.5,
					},
				},
			},
			{
				Name:       "Human",
				Confidence: 99.5,
				Parents:    []Parent{},
				Categories: []LabelCategory{{Name: "Person Description"}},
			},
			{
				Name:       "Face",
				Confidence: 98.8,
				Parents:    []Parent{{Name: "Person"}, {Name: "Human"}},
				Categories: []LabelCategory{{Name: "Person Description"}},
			},
			{
				Name:       "Outdoors",
				Confidence: 85.3,
				Parents:    []Parent{},
				Categories: []LabelCategory{{Name: "Places and Locations"}},
			},
			{
				Name:       "Nature",
				Confidence: 82.1,
				Parents:    []Parent{{Name: "Outdoors"}},
				Categories: []LabelCategory{{Name: "Places and Locations"}},
			},
		},
	}, nil
}

// DetectText detects text in an image.
func (s *MemoryStorage) DetectText(_ context.Context, _ *DetectTextRequest) (*DetectTextResponse, error) {
	// Return mock text detection results
	parentID := 0

	return &DetectTextResponse{
		TextModelVersion: defaultTextModelVersion,
		TextDetections: []TextDetection{
			{
				ID:           0,
				DetectedText: "HELLO WORLD",
				Type:         "LINE",
				Confidence:   99.5,
				Geometry: &Geometry{
					BoundingBox: &BoundingBox{Height: 0.08, Left: 0.1, Top: 0.2, Width: 0.3},
					Polygon: []Point{
						{X: 0.1, Y: 0.2},
						{X: 0.4, Y: 0.2},
						{X: 0.4, Y: 0.28},
						{X: 0.1, Y: 0.28},
					},
				},
			},
			{
				ID:           1,
				ParentID:     &parentID,
				DetectedText: "HELLO",
				Type:         "WORD",
				Confidence:   99.8,
				Geometry: &Geometry{
					BoundingBox: &BoundingBox{Height: 0.08, Left: 0.1, Top: 0.2, Width: 0.12},
					Polygon: []Point{
						{X: 0.1, Y: 0.2},
						{X: 0.22, Y: 0.2},
						{X: 0.22, Y: 0.28},
						{X: 0.1, Y: 0.28},
					},
				},
			},
			{
				ID:           2,
				ParentID:     &parentID,
				DetectedText: "WORLD",
				Type:         "WORD",
				Confidence:   99.6,
				Geometry: &Geometry{
					BoundingBox: &BoundingBox{Height: 0.08, Left: 0.25, Top: 0.2, Width: 0.15},
					Polygon: []Point{
						{X: 0.25, Y: 0.2},
						{X: 0.4, Y: 0.2},
						{X: 0.4, Y: 0.28},
						{X: 0.25, Y: 0.28},
					},
				},
			},
		},
	}, nil
}

// RecognizeCelebrities recognizes celebrities in an image.
func (s *MemoryStorage) RecognizeCelebrities(_ context.Context, _ *RecognizeCelebritiesRequest) (*RecognizeCelebritiesResponse, error) {
	// Return mock celebrity recognition results (no celebrities found for mock)
	return &RecognizeCelebritiesResponse{
		CelebrityFaces:    []Celebrity{},
		UnrecognizedFaces: []FaceDetail{},
	}, nil
}

// DetectModerationLabels detects moderation labels in an image.
func (s *MemoryStorage) DetectModerationLabels(_ context.Context, _ *DetectModerationLabelsRequest) (*DetectModerationLabelsResponse, error) {
	// Return mock moderation results (safe content)
	return &DetectModerationLabelsResponse{
		ModerationModelVersion: defaultModerationModelVersion,
		ModerationLabels:       []ModerationLabel{},
		ContentTypes: []ContentType{
			{Name: "Illustrated", Confidence: 75.5},
		},
	}, nil
}

// generateMockLandmarks generates mock facial landmarks.
func generateMockLandmarks() []Landmark {
	return []Landmark{
		{Type: "eyeLeft", X: 0.35, Y: 0.35},
		{Type: "eyeRight", X: 0.65, Y: 0.35},
		{Type: "mouthLeft", X: 0.35, Y: 0.7},
		{Type: "mouthRight", X: 0.65, Y: 0.7},
		{Type: "nose", X: 0.5, Y: 0.55},
		{Type: "leftEyeBrowLeft", X: 0.28, Y: 0.3},
		{Type: "leftEyeBrowRight", X: 0.38, Y: 0.28},
		{Type: "leftEyeBrowUp", X: 0.33, Y: 0.27},
		{Type: "rightEyeBrowLeft", X: 0.62, Y: 0.28},
		{Type: "rightEyeBrowRight", X: 0.72, Y: 0.3},
		{Type: "rightEyeBrowUp", X: 0.67, Y: 0.27},
		{Type: "leftEyeLeft", X: 0.32, Y: 0.35},
		{Type: "leftEyeRight", X: 0.38, Y: 0.35},
		{Type: "leftEyeUp", X: 0.35, Y: 0.33},
		{Type: "leftEyeDown", X: 0.35, Y: 0.37},
		{Type: "rightEyeLeft", X: 0.62, Y: 0.35},
		{Type: "rightEyeRight", X: 0.68, Y: 0.35},
		{Type: "rightEyeUp", X: 0.65, Y: 0.33},
		{Type: "rightEyeDown", X: 0.65, Y: 0.37},
		{Type: "noseLeft", X: 0.45, Y: 0.6},
		{Type: "noseRight", X: 0.55, Y: 0.6},
		{Type: "mouthUp", X: 0.5, Y: 0.68},
		{Type: "mouthDown", X: 0.5, Y: 0.75},
		{Type: "leftPupil", X: 0.35, Y: 0.35},
		{Type: "rightPupil", X: 0.65, Y: 0.35},
		{Type: "upperJawlineLeft", X: 0.2, Y: 0.4},
		{Type: "midJawlineLeft", X: 0.22, Y: 0.55},
		{Type: "chinBottom", X: 0.5, Y: 0.85},
		{Type: "midJawlineRight", X: 0.78, Y: 0.55},
		{Type: "upperJawlineRight", X: 0.8, Y: 0.4},
	}
}
