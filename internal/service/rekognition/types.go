// Package rekognition provides AWS Rekognition service emulation.
package rekognition

import "github.com/sivchari/kumo/internal/service"

// Image represents an image for Rekognition operations.
type Image struct {
	Bytes    []byte    `json:"Bytes,omitempty"`
	S3Object *S3Object `json:"S3Object,omitempty"`
}

// S3Object represents an S3 object reference.
type S3Object struct {
	Bucket  string `json:"Bucket,omitempty"`
	Name    string `json:"Name,omitempty"`
	Version string `json:"Version,omitempty"`
}

// BoundingBox represents a bounding box around a detected object.
type BoundingBox struct {
	Height float64 `json:"Height"`
	Left   float64 `json:"Left"`
	Top    float64 `json:"Top"`
	Width  float64 `json:"Width"`
}

// Landmark represents a facial landmark.
type Landmark struct {
	Type string  `json:"Type"`
	X    float64 `json:"X"`
	Y    float64 `json:"Y"`
}

// Pose represents the pose of a face.
type Pose struct {
	Pitch float64 `json:"Pitch"`
	Roll  float64 `json:"Roll"`
	Yaw   float64 `json:"Yaw"`
}

// ImageQuality represents the quality of an image.
type ImageQuality struct {
	Brightness float64 `json:"Brightness"`
	Sharpness  float64 `json:"Sharpness"`
}

// AgeRange represents an estimated age range.
type AgeRange struct {
	High int `json:"High"`
	Low  int `json:"Low"`
}

// Attribute represents a boolean attribute with confidence.
type Attribute struct {
	Confidence float64 `json:"Confidence"`
	Value      bool    `json:"Value"`
}

// Gender represents detected gender.
type Gender struct {
	Confidence float64 `json:"Confidence"`
	Value      string  `json:"Value"`
}

// Emotion represents a detected emotion.
type Emotion struct {
	Confidence float64 `json:"Confidence"`
	Type       string  `json:"Type"`
}

// FaceDetail represents detailed face information.
type FaceDetail struct {
	AgeRange    *AgeRange     `json:"AgeRange,omitempty"`
	Beard       *Attribute    `json:"Beard,omitempty"`
	BoundingBox *BoundingBox  `json:"BoundingBox,omitempty"`
	Confidence  float64       `json:"Confidence,omitempty"`
	Emotions    []Emotion     `json:"Emotions,omitempty"`
	Eyeglasses  *Attribute    `json:"Eyeglasses,omitempty"`
	EyesOpen    *Attribute    `json:"EyesOpen,omitempty"`
	Gender      *Gender       `json:"Gender,omitempty"`
	Landmarks   []Landmark    `json:"Landmarks,omitempty"`
	MouthOpen   *Attribute    `json:"MouthOpen,omitempty"`
	Mustache    *Attribute    `json:"Mustache,omitempty"`
	Pose        *Pose         `json:"Pose,omitempty"`
	Quality     *ImageQuality `json:"Quality,omitempty"`
	Smile       *Attribute    `json:"Smile,omitempty"`
	Sunglasses  *Attribute    `json:"Sunglasses,omitempty"`
}

// Face represents a face in a collection.
type Face struct {
	BoundingBox            *BoundingBox `json:"BoundingBox,omitempty"`
	Confidence             float64      `json:"Confidence,omitempty"`
	ExternalImageID        string       `json:"ExternalImageId,omitempty"`
	FaceID                 string       `json:"FaceId,omitempty"`
	ImageID                string       `json:"ImageId,omitempty"`
	IndexFacesModelVersion string       `json:"IndexFacesModelVersion,omitempty"`
	UserID                 string       `json:"UserId,omitempty"`
}

// FaceMatch represents a face match result.
type FaceMatch struct {
	Face       *Face   `json:"Face,omitempty"`
	Similarity float64 `json:"Similarity,omitempty"`
}

// FaceRecord represents an indexed face record.
type FaceRecord struct {
	Face       *Face       `json:"Face,omitempty"`
	FaceDetail *FaceDetail `json:"FaceDetail,omitempty"`
}

// UnindexedFace represents a face that could not be indexed.
type UnindexedFace struct {
	FaceDetail *FaceDetail `json:"FaceDetail,omitempty"`
	Reasons    []string    `json:"Reasons,omitempty"`
}

// Label represents a detected label.
type Label struct {
	Aliases    []LabelAlias    `json:"Aliases,omitempty"`
	Categories []LabelCategory `json:"Categories,omitempty"`
	Confidence float64         `json:"Confidence,omitempty"`
	Instances  []Instance      `json:"Instances,omitempty"`
	Name       string          `json:"Name,omitempty"`
	Parents    []Parent        `json:"Parents,omitempty"`
}

// LabelAlias represents a label alias.
type LabelAlias struct {
	Name string `json:"Name,omitempty"`
}

// LabelCategory represents a label category.
type LabelCategory struct {
	Name string `json:"Name,omitempty"`
}

// Instance represents an instance of a label.
type Instance struct {
	BoundingBox    *BoundingBox    `json:"BoundingBox,omitempty"`
	Confidence     float64         `json:"Confidence,omitempty"`
	DominantColors []DominantColor `json:"DominantColors,omitempty"`
}

// DominantColor represents a dominant color.
type DominantColor struct {
	Blue            int     `json:"Blue,omitempty"`
	CSSColor        string  `json:"CSSColor,omitempty"`
	Green           int     `json:"Green,omitempty"`
	HexCode         string  `json:"HexCode,omitempty"`
	PixelPercent    float64 `json:"PixelPercent,omitempty"`
	Red             int     `json:"Red,omitempty"`
	SimplifiedColor string  `json:"SimplifiedColor,omitempty"`
}

// Parent represents a parent label.
type Parent struct {
	Name string `json:"Name,omitempty"`
}

// TextDetection represents detected text.
type TextDetection struct {
	Confidence   float64   `json:"Confidence,omitempty"`
	DetectedText string    `json:"DetectedText,omitempty"`
	Geometry     *Geometry `json:"Geometry,omitempty"`
	ID           int       `json:"Id,omitempty"`
	ParentID     *int      `json:"ParentId,omitempty"`
	Type         string    `json:"Type,omitempty"`
}

// Geometry represents text geometry.
type Geometry struct {
	BoundingBox *BoundingBox `json:"BoundingBox,omitempty"`
	Polygon     []Point      `json:"Polygon,omitempty"`
}

// Point represents a point in a polygon.
type Point struct {
	X float64 `json:"X"`
	Y float64 `json:"Y"`
}

// Celebrity represents a recognized celebrity.
type Celebrity struct {
	Face            *FaceDetail  `json:"Face,omitempty"`
	ID              string       `json:"Id,omitempty"`
	KnownGender     *KnownGender `json:"KnownGender,omitempty"`
	MatchConfidence float64      `json:"MatchConfidence,omitempty"`
	Name            string       `json:"Name,omitempty"`
	Urls            []string     `json:"Urls,omitempty"`
}

// KnownGender represents known gender information.
type KnownGender struct {
	Type string `json:"Type,omitempty"`
}

// ModerationLabel represents a moderation label.
type ModerationLabel struct {
	Confidence    float64 `json:"Confidence,omitempty"`
	Name          string  `json:"Name,omitempty"`
	ParentName    string  `json:"ParentName,omitempty"`
	TaxonomyLevel int     `json:"TaxonomyLevel,omitempty"`
}

// ContentType represents content type information.
type ContentType struct {
	Confidence float64 `json:"Confidence,omitempty"`
	Name       string  `json:"Name,omitempty"`
}

// CreateCollectionRequest represents a CreateCollection request.
type CreateCollectionRequest struct {
	CollectionID string            `json:"CollectionId"`
	Tags         map[string]string `json:"Tags,omitempty"`
}

// CreateCollectionResponse represents a CreateCollection response.
type CreateCollectionResponse struct {
	CollectionArn    string `json:"CollectionArn,omitempty"`
	FaceModelVersion string `json:"FaceModelVersion,omitempty"`
	StatusCode       int    `json:"StatusCode,omitempty"`
}

// DeleteCollectionRequest represents a DeleteCollection request.
type DeleteCollectionRequest struct {
	CollectionID string `json:"CollectionId"`
}

// DeleteCollectionResponse represents a DeleteCollection response.
type DeleteCollectionResponse struct {
	StatusCode int `json:"StatusCode,omitempty"`
}

// ListCollectionsRequest represents a ListCollections request.
type ListCollectionsRequest struct {
	MaxResults int    `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

// ListCollectionsResponse represents a ListCollections response.
type ListCollectionsResponse struct {
	CollectionIDs     []string `json:"CollectionIds,omitempty"`
	FaceModelVersions []string `json:"FaceModelVersions,omitempty"`
	NextToken         string   `json:"NextToken,omitempty"`
}

// DetectFacesRequest represents a DetectFaces request.
type DetectFacesRequest struct {
	Attributes []string `json:"Attributes,omitempty"`
	Image      Image    `json:"Image"`
}

// DetectFacesResponse represents a DetectFaces response.
type DetectFacesResponse struct {
	FaceDetails           []FaceDetail `json:"FaceDetails,omitempty"`
	OrientationCorrection string       `json:"OrientationCorrection,omitempty"`
}

// IndexFacesRequest represents an IndexFaces request.
type IndexFacesRequest struct {
	CollectionID        string   `json:"CollectionId"`
	DetectionAttributes []string `json:"DetectionAttributes,omitempty"`
	ExternalImageID     string   `json:"ExternalImageId,omitempty"`
	Image               Image    `json:"Image"`
	MaxFaces            int      `json:"MaxFaces,omitempty"`
	QualityFilter       string   `json:"QualityFilter,omitempty"`
}

// IndexFacesResponse represents an IndexFaces response.
type IndexFacesResponse struct {
	FaceModelVersion      string          `json:"FaceModelVersion,omitempty"`
	FaceRecords           []FaceRecord    `json:"FaceRecords,omitempty"`
	OrientationCorrection string          `json:"OrientationCorrection,omitempty"`
	UnindexedFaces        []UnindexedFace `json:"UnindexedFaces,omitempty"`
}

// SearchFacesRequest represents a SearchFaces request.
type SearchFacesRequest struct {
	CollectionID       string  `json:"CollectionId"`
	FaceID             string  `json:"FaceId"`
	FaceMatchThreshold float64 `json:"FaceMatchThreshold,omitempty"`
	MaxFaces           int     `json:"MaxFaces,omitempty"`
}

// SearchFacesResponse represents a SearchFaces response.
type SearchFacesResponse struct {
	FaceMatches      []FaceMatch `json:"FaceMatches,omitempty"`
	FaceModelVersion string      `json:"FaceModelVersion,omitempty"`
	SearchedFaceID   string      `json:"SearchedFaceId,omitempty"`
}

// ListFacesRequest represents a ListFaces request.
type ListFacesRequest struct {
	CollectionID string   `json:"CollectionId"`
	FaceIDs      []string `json:"FaceIds,omitempty"`
	MaxResults   int      `json:"MaxResults,omitempty"`
	NextToken    string   `json:"NextToken,omitempty"`
	UserID       string   `json:"UserId,omitempty"`
}

// ListFacesResponse represents a ListFaces response.
type ListFacesResponse struct {
	FaceModelVersion string `json:"FaceModelVersion,omitempty"`
	Faces            []Face `json:"Faces,omitempty"`
	NextToken        string `json:"NextToken,omitempty"`
}

// DeleteFacesRequest represents a DeleteFaces request.
type DeleteFacesRequest struct {
	CollectionID string   `json:"CollectionId"`
	FaceIDs      []string `json:"FaceIds"`
}

// DeleteFacesResponse represents a DeleteFaces response.
type DeleteFacesResponse struct {
	DeletedFaces              []string                   `json:"DeletedFaces,omitempty"`
	UnsuccessfulFaceDeletions []UnsuccessfulFaceDeletion `json:"UnsuccessfulFaceDeletions,omitempty"`
}

// UnsuccessfulFaceDeletion represents an unsuccessful face deletion.
type UnsuccessfulFaceDeletion struct {
	FaceID  string   `json:"FaceId,omitempty"`
	Reasons []string `json:"Reasons,omitempty"`
	UserID  string   `json:"UserId,omitempty"`
}

// DetectLabelsRequest represents a DetectLabels request.
type DetectLabelsRequest struct {
	Features      []string              `json:"Features,omitempty"`
	Image         Image                 `json:"Image"`
	MaxLabels     int                   `json:"MaxLabels,omitempty"`
	MinConfidence float64               `json:"MinConfidence,omitempty"`
	Settings      *DetectLabelsSettings `json:"Settings,omitempty"`
}

// DetectLabelsSettings represents settings for DetectLabels.
type DetectLabelsSettings struct {
	GeneralLabels   *GeneralLabelsSettings   `json:"GeneralLabels,omitempty"`
	ImageProperties *ImagePropertiesSettings `json:"ImageProperties,omitempty"`
}

// GeneralLabelsSettings represents general labels settings.
type GeneralLabelsSettings struct {
	LabelCategoryExclusionFilters []string `json:"LabelCategoryExclusionFilters,omitempty"`
	LabelCategoryInclusionFilters []string `json:"LabelCategoryInclusionFilters,omitempty"`
	LabelExclusionFilters         []string `json:"LabelExclusionFilters,omitempty"`
	LabelInclusionFilters         []string `json:"LabelInclusionFilters,omitempty"`
}

// ImagePropertiesSettings represents image properties settings.
type ImagePropertiesSettings struct {
	MaxDominantColors int `json:"MaxDominantColors,omitempty"`
}

// DetectLabelsResponse represents a DetectLabels response.
type DetectLabelsResponse struct {
	ImageProperties       *ImageProperties `json:"ImageProperties,omitempty"`
	LabelModelVersion     string           `json:"LabelModelVersion,omitempty"`
	Labels                []Label          `json:"Labels,omitempty"`
	OrientationCorrection string           `json:"OrientationCorrection,omitempty"`
}

// ImageProperties represents image properties.
type ImageProperties struct {
	Background     *BackgroundColors `json:"Background,omitempty"`
	DominantColors []DominantColor   `json:"DominantColors,omitempty"`
	Foreground     *ForegroundColors `json:"Foreground,omitempty"`
	Quality        *ImageQuality     `json:"Quality,omitempty"`
}

// BackgroundColors represents background colors.
type BackgroundColors struct {
	DominantColors []DominantColor `json:"DominantColors,omitempty"`
}

// ForegroundColors represents foreground colors.
type ForegroundColors struct {
	DominantColors []DominantColor `json:"DominantColors,omitempty"`
}

// DetectTextRequest represents a DetectText request.
type DetectTextRequest struct {
	Filters *DetectTextFilters `json:"Filters,omitempty"`
	Image   Image              `json:"Image"`
}

// DetectTextFilters represents filters for DetectText.
type DetectTextFilters struct {
	RegionsOfInterest []RegionOfInterest `json:"RegionsOfInterest,omitempty"`
	WordFilter        *WordFilter        `json:"WordFilter,omitempty"`
}

// RegionOfInterest represents a region of interest.
type RegionOfInterest struct {
	BoundingBox *BoundingBox `json:"BoundingBox,omitempty"`
	Polygon     []Point      `json:"Polygon,omitempty"`
}

// WordFilter represents a word filter.
type WordFilter struct {
	MinBoundingBoxHeight float64 `json:"MinBoundingBoxHeight,omitempty"`
	MinBoundingBoxWidth  float64 `json:"MinBoundingBoxWidth,omitempty"`
	MinConfidence        float64 `json:"MinConfidence,omitempty"`
}

// DetectTextResponse represents a DetectText response.
type DetectTextResponse struct {
	TextDetections   []TextDetection `json:"TextDetections,omitempty"`
	TextModelVersion string          `json:"TextModelVersion,omitempty"`
}

// RecognizeCelebritiesRequest represents a RecognizeCelebrities request.
type RecognizeCelebritiesRequest struct {
	Image Image `json:"Image"`
}

// RecognizeCelebritiesResponse represents a RecognizeCelebrities response.
type RecognizeCelebritiesResponse struct {
	CelebrityFaces        []Celebrity  `json:"CelebrityFaces,omitempty"`
	OrientationCorrection string       `json:"OrientationCorrection,omitempty"`
	UnrecognizedFaces     []FaceDetail `json:"UnrecognizedFaces,omitempty"`
}

// DetectModerationLabelsRequest represents a DetectModerationLabels request.
type DetectModerationLabelsRequest struct {
	HumanLoopConfig *HumanLoopConfig `json:"HumanLoopConfig,omitempty"`
	Image           Image            `json:"Image"`
	MinConfidence   float64          `json:"MinConfidence,omitempty"`
	ProjectVersion  string           `json:"ProjectVersion,omitempty"`
}

// HumanLoopConfig represents human loop configuration.
type HumanLoopConfig struct {
	DataAttributes    *HumanLoopDataAttributes `json:"DataAttributes,omitempty"`
	FlowDefinitionArn string                   `json:"FlowDefinitionArn,omitempty"`
	HumanLoopName     string                   `json:"HumanLoopName,omitempty"`
}

// HumanLoopDataAttributes represents human loop data attributes.
type HumanLoopDataAttributes struct {
	ContentClassifiers []string `json:"ContentClassifiers,omitempty"`
}

// DetectModerationLabelsResponse represents a DetectModerationLabels response.
type DetectModerationLabelsResponse struct {
	ContentTypes              []ContentType              `json:"ContentTypes,omitempty"`
	HumanLoopActivationOutput *HumanLoopActivationOutput `json:"HumanLoopActivationOutput,omitempty"`
	ModerationLabels          []ModerationLabel          `json:"ModerationLabels,omitempty"`
	ModerationModelVersion    string                     `json:"ModerationModelVersion,omitempty"`
}

// HumanLoopActivationOutput represents human loop activation output.
type HumanLoopActivationOutput struct {
	HumanLoopActivationConditionsEvaluationResults string   `json:"HumanLoopActivationConditionsEvaluationResults,omitempty"`
	HumanLoopActivationReasons                     []string `json:"HumanLoopActivationReasons,omitempty"`
	HumanLoopArn                                   string   `json:"HumanLoopArn,omitempty"`
}

// DescribeCollectionRequest represents a DescribeCollection request.
type DescribeCollectionRequest struct {
	CollectionID string `json:"CollectionId"`
}

// DescribeCollectionResponse represents a DescribeCollection response.
type DescribeCollectionResponse struct {
	CollectionARN     string  `json:"CollectionARN,omitempty"`
	CreationTimestamp float64 `json:"CreationTimestamp,omitempty"`
	FaceCount         int64   `json:"FaceCount,omitempty"`
	FaceModelVersion  string  `json:"FaceModelVersion,omitempty"`
	UserCount         int64   `json:"UserCount,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a service error.
type ServiceError = service.CodedError

// Error returns the error message.
// Error codes.
const (
	errResourceNotFound      = "ResourceNotFoundException"
	errResourceExists        = "ResourceAlreadyExistsException"
	errInvalidParameter      = "InvalidParameterException"
	errAccessDenied          = "AccessDeniedException"
	errInternalServer        = "InternalServerError"
	errImageTooLarge         = "ImageTooLargeException"
	errInvalidImageFormat    = "InvalidImageFormatException"
	errProvisionedThroughput = "ProvisionedThroughputExceededException"
	errThrottling            = "ThrottlingException"
)
