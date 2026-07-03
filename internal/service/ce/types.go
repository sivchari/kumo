package ce

import "github.com/sivchari/kumo/internal/service"

// DateInterval represents a time period.
type DateInterval struct {
	Start string `json:"Start"`
	End   string `json:"End"`
}

// GroupDefinition represents how to group results.
type GroupDefinition struct {
	Type string `json:"Type"`
	Key  string `json:"Key"`
}

// MetricValue represents a metric value.
type MetricValue struct {
	Amount string `json:"Amount"`
	Unit   string `json:"Unit"`
}

// Group represents a group in cost results.
type Group struct {
	Keys    []string               `json:"Keys"`
	Metrics map[string]MetricValue `json:"Metrics"`
}

// ResultByTime represents cost results for a time period.
type ResultByTime struct {
	TimePeriod DateInterval           `json:"TimePeriod"`
	Total      map[string]MetricValue `json:"Total,omitempty"`
	Groups     []Group                `json:"Groups,omitempty"`
	Estimated  bool                   `json:"Estimated"`
}

// Expression represents a filter expression.
type Expression struct {
	Dimensions   *DimensionValues `json:"Dimensions,omitempty"`
	Tags         *TagValues       `json:"Tags,omitempty"`
	CostCategory *CostCategory    `json:"CostCategories,omitempty"`
	And          []Expression     `json:"And,omitempty"`
	Or           []Expression     `json:"Or,omitempty"`
	Not          *Expression      `json:"Not,omitempty"`
}

// DimensionValues represents dimension filter values.
type DimensionValues struct {
	Key          string   `json:"Key"`
	Values       []string `json:"Values,omitempty"`
	MatchOptions []string `json:"MatchOptions,omitempty"`
}

// TagValues represents tag filter values.
type TagValues struct {
	Key          string   `json:"Key"`
	Values       []string `json:"Values,omitempty"`
	MatchOptions []string `json:"MatchOptions,omitempty"`
}

// CostCategory represents cost category filter.
type CostCategory struct {
	Key          string   `json:"Key"`
	Values       []string `json:"Values,omitempty"`
	MatchOptions []string `json:"MatchOptions,omitempty"`
}

// GetCostAndUsageRequest is the request for GetCostAndUsage.
type GetCostAndUsageRequest struct {
	TimePeriod  DateInterval      `json:"TimePeriod"`
	Granularity string            `json:"Granularity"`
	Filter      *Expression       `json:"Filter,omitempty"`
	Metrics     []string          `json:"Metrics,omitempty"`
	GroupBy     []GroupDefinition `json:"GroupBy,omitempty"`
	NextToken   string            `json:"NextPageToken,omitempty"`
}

// GetCostAndUsageResponse is the response for GetCostAndUsage.
type GetCostAndUsageResponse struct {
	ResultsByTime        []ResultByTime    `json:"ResultsByTime"`
	GroupDefinitions     []GroupDefinition `json:"GroupDefinitions,omitempty"`
	DimensionValueAttrs  []string          `json:"DimensionValueAttributes,omitempty"`
	NextPageToken        string            `json:"NextPageToken,omitempty"`
	BillingViewArn       string            `json:"BillingViewArn,omitempty"`
	ApproximateUsageDate string            `json:"ApproximateUsageDate,omitempty"`
}

// DimensionValueItem represents a dimension value.
type DimensionValueItem struct {
	Value      string            `json:"Value"`
	Attributes map[string]string `json:"Attributes,omitempty"`
}

// GetDimensionValuesRequest is the request for GetDimensionValues.
type GetDimensionValuesRequest struct {
	TimePeriod   DateInterval `json:"TimePeriod"`
	Dimension    string       `json:"Dimension"`
	Context      string       `json:"Context,omitempty"`
	Filter       *Expression  `json:"Filter,omitempty"`
	SearchString string       `json:"SearchString,omitempty"`
	SortBy       []SortBy     `json:"SortBy,omitempty"`
	MaxResults   int          `json:"MaxResults,omitempty"`
	NextToken    string       `json:"NextPageToken,omitempty"`
}

// SortBy represents sort options.
type SortBy struct {
	Key       string `json:"Key"`
	SortOrder string `json:"SortOrder,omitempty"`
}

// GetDimensionValuesResponse is the response for GetDimensionValues.
type GetDimensionValuesResponse struct {
	DimensionValues []DimensionValueItem `json:"DimensionValues"`
	ReturnSize      int                  `json:"ReturnSize"`
	TotalSize       int                  `json:"TotalSize"`
	NextPageToken   string               `json:"NextPageToken,omitempty"`
}

// TagItem represents a tag value.
type TagItem struct {
	Key   string   `json:"Key,omitempty"`
	Value string   `json:"Value,omitempty"`
	Types []string `json:"Types,omitempty"`
}

// GetTagsRequest is the request for GetTags.
type GetTagsRequest struct {
	TimePeriod   DateInterval `json:"TimePeriod"`
	TagKey       string       `json:"TagKey,omitempty"`
	Filter       *Expression  `json:"Filter,omitempty"`
	SearchString string       `json:"SearchString,omitempty"`
	SortBy       []SortBy     `json:"SortBy,omitempty"`
	MaxResults   int          `json:"MaxResults,omitempty"`
	NextToken    string       `json:"NextPageToken,omitempty"`
}

// GetTagsResponse is the response for GetTags.
type GetTagsResponse struct {
	Tags          []string `json:"Tags"`
	ReturnSize    int      `json:"ReturnSize"`
	TotalSize     int      `json:"TotalSize"`
	NextPageToken string   `json:"NextPageToken,omitempty"`
}

// ForecastResult represents a forecast for a time period.
type ForecastResult struct {
	TimePeriod                   DateInterval `json:"TimePeriod"`
	MeanValue                    string       `json:"MeanValue,omitempty"`
	PredictionIntervalLowerBound string       `json:"PredictionIntervalLowerBound,omitempty"`
	PredictionIntervalUpperBound string       `json:"PredictionIntervalUpperBound,omitempty"`
}

// GetCostForecastRequest is the request for GetCostForecast.
type GetCostForecastRequest struct {
	TimePeriod         DateInterval `json:"TimePeriod"`
	Metric             string       `json:"Metric"`
	Granularity        string       `json:"Granularity"`
	Filter             *Expression  `json:"Filter,omitempty"`
	PredictionInterval int          `json:"PredictionIntervalLevel,omitempty"`
}

// GetCostForecastResponse is the response for GetCostForecast.
type GetCostForecastResponse struct {
	Total            MetricValue      `json:"Total,omitempty"`
	ForecastResultsT []ForecastResult `json:"ForecastResultsByTime,omitempty"`
}

// CostCategoryRule represents a cost category rule.
type CostCategoryRule struct {
	Value string      `json:"Value,omitempty"`
	Rule  *Expression `json:"Rule,omitempty"`
	Type  string      `json:"Type,omitempty"`
}

// CostCategorySplitChargeRule represents a split charge rule.
type CostCategorySplitChargeRule struct {
	Source     string                             `json:"Source,omitempty"`
	Targets    []string                           `json:"Targets,omitempty"`
	Method     string                             `json:"Method,omitempty"`
	Parameters []CostCategorySplitChargeParameter `json:"Parameters,omitempty"`
}

// CostCategorySplitChargeParameter represents a parameter for split charge.
type CostCategorySplitChargeParameter struct {
	Type   string   `json:"Type,omitempty"`
	Values []string `json:"Values,omitempty"`
}

// CreateCostCategoryDefinitionRequest is the request for CreateCostCategoryDefinition.
type CreateCostCategoryDefinitionRequest struct {
	Name             string                        `json:"Name"`
	RuleVersion      string                        `json:"RuleVersion"`
	Rules            []CostCategoryRule            `json:"Rules"`
	DefaultValue     string                        `json:"DefaultValue,omitempty"`
	SplitChargeRules []CostCategorySplitChargeRule `json:"SplitChargeRules,omitempty"`
	EffectiveStart   string                        `json:"EffectiveStart,omitempty"`
	Tags             []ResourceTag                 `json:"ResourceTags,omitempty"`
}

// ResourceTag represents a resource tag.
type ResourceTag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// CreateCostCategoryDefinitionResponse is the response for CreateCostCategoryDefinition.
type CreateCostCategoryDefinitionResponse struct {
	CostCategoryArn string `json:"CostCategoryArn"`
	EffectiveStart  string `json:"EffectiveStart,omitempty"`
}

// DescribeCostCategoryDefinitionRequest is the request for DescribeCostCategoryDefinition.
type DescribeCostCategoryDefinitionRequest struct {
	CostCategoryArn string `json:"CostCategoryArn"`
	EffectiveOn     string `json:"EffectiveOn,omitempty"`
}

// CostCategoryDefinition represents a cost category definition.
type CostCategoryDefinition struct {
	CostCategoryArn  string                         `json:"CostCategoryArn"`
	Name             string                         `json:"Name"`
	RuleVersion      string                         `json:"RuleVersion"`
	Rules            []CostCategoryRule             `json:"Rules"`
	DefaultValue     string                         `json:"DefaultValue,omitempty"`
	SplitChargeRules []CostCategorySplitChargeRule  `json:"SplitChargeRules,omitempty"`
	EffectiveStart   string                         `json:"EffectiveStart,omitempty"`
	EffectiveEnd     string                         `json:"EffectiveEnd,omitempty"`
	ProcessingStatus []CostCategoryProcessingStatus `json:"ProcessingStatus,omitempty"`
}

// CostCategoryProcessingStatus represents processing status.
type CostCategoryProcessingStatus struct {
	Component string `json:"Component,omitempty"`
	Status    string `json:"Status,omitempty"`
}

// DescribeCostCategoryDefinitionResponse is the response for DescribeCostCategoryDefinition.
type DescribeCostCategoryDefinitionResponse struct {
	CostCategory *CostCategoryDefinition `json:"CostCategory"`
}

// DeleteCostCategoryDefinitionRequest is the request for DeleteCostCategoryDefinition.
type DeleteCostCategoryDefinitionRequest struct {
	CostCategoryArn string `json:"CostCategoryArn"`
}

// DeleteCostCategoryDefinitionResponse is the response for DeleteCostCategoryDefinition.
type DeleteCostCategoryDefinitionResponse struct {
	CostCategoryArn string `json:"CostCategoryArn,omitempty"`
	EffectiveEnd    string `json:"EffectiveEnd,omitempty"`
}

// ListCostCategoryDefinitionsRequest is the request for ListCostCategoryDefinitions.
type ListCostCategoryDefinitionsRequest struct {
	EffectiveOn string `json:"EffectiveOn,omitempty"`
	NextToken   string `json:"NextToken,omitempty"`
	MaxResults  int    `json:"MaxResults,omitempty"`
}

// CostCategoryReference represents a cost category reference.
type CostCategoryReference struct {
	CostCategoryArn  string                         `json:"CostCategoryArn,omitempty"`
	Name             string                         `json:"Name,omitempty"`
	EffectiveStart   string                         `json:"EffectiveStart,omitempty"`
	EffectiveEnd     string                         `json:"EffectiveEnd,omitempty"`
	NumberOfRules    int                            `json:"NumberOfRules,omitempty"`
	ProcessingStatus []CostCategoryProcessingStatus `json:"ProcessingStatus,omitempty"`
	Values           []string                       `json:"Values,omitempty"`
	DefaultValue     string                         `json:"DefaultValue,omitempty"`
}

// ListCostCategoryDefinitionsResponse is the response for ListCostCategoryDefinitions.
type ListCostCategoryDefinitionsResponse struct {
	CostCategoryReferences []CostCategoryReference `json:"CostCategoryReferences,omitempty"`
	NextToken              string                  `json:"NextToken,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a service error.
type ServiceError = service.CodedError

// Error codes.
const (
	errNotFound           = "ResourceNotFoundException"
	errValidation         = "ValidationException"
	errDataUnavailable    = "DataUnavailableException"
	errBillExpiration     = "BillExpirationException"
	errLimitExceeded      = "LimitExceededException"
	errInvalidNextToken   = "InvalidNextTokenException"
	errServiceQuota       = "ServiceQuotaExceededException"
	errBackfillInProgress = "BackfillLimitExceededException"
)
