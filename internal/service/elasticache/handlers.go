// Package elasticache implements the ElastiCache service handlers.
package elasticache

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

const elasticacheXMLNS = "http://elasticache.amazonaws.com/doc/2015-02-02/"

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)

	switch action {
	case "CreateCacheCluster":
		s.CreateCacheCluster(w, r)
	case "DeleteCacheCluster":
		s.DeleteCacheCluster(w, r)
	case "DescribeCacheClusters":
		s.DescribeCacheClusters(w, r)
	case "ModifyCacheCluster":
		s.ModifyCacheCluster(w, r)
	case "CreateReplicationGroup":
		s.CreateReplicationGroup(w, r)
	case "DeleteReplicationGroup":
		s.DeleteReplicationGroup(w, r)
	case "DescribeReplicationGroups":
		s.DescribeReplicationGroups(w, r)
	default:
		writeError(w, errInvalidParameterValue, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)
	}
}

// CreateCacheCluster handles the CreateCacheCluster action.
func (s *Service) CreateCacheCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateCacheClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.CacheClusterID == "" {
		writeError(w, errInvalidParameterValue, "CacheClusterId is required", http.StatusBadRequest)

		return
	}

	if req.CacheNodeType == "" {
		writeError(w, errInvalidParameterValue, "CacheNodeType is required", http.StatusBadRequest)

		return
	}

	if req.Engine == "" {
		writeError(w, errInvalidParameterValue, "Engine is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.CreateCacheCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateCacheClusterResponse{
		Xmlns:        elasticacheXMLNS,
		CacheCluster: convertToXMLCacheCluster(cluster),
		RequestID:    uuid.New().String(),
	})
}

// DeleteCacheCluster handles the DeleteCacheCluster action.
func (s *Service) DeleteCacheCluster(w http.ResponseWriter, r *http.Request) {
	var req DeleteCacheClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.CacheClusterID == "" {
		writeError(w, errInvalidParameterValue, "CacheClusterId is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.DeleteCacheCluster(r.Context(), req.CacheClusterID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLDeleteCacheClusterResponse{
		Xmlns:        elasticacheXMLNS,
		CacheCluster: convertToXMLCacheCluster(cluster),
		RequestID:    uuid.New().String(),
	})
}

// DescribeCacheClusters handles the DescribeCacheClusters action.
func (s *Service) DescribeCacheClusters(w http.ResponseWriter, r *http.Request) {
	var req DescribeCacheClustersInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	clusters, err := s.storage.DescribeCacheClusters(r.Context(), req.CacheClusterID, req.ShowCacheNodeInfo)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlClusters := make([]XMLCacheCluster, 0, len(clusters))
	for i := range clusters {
		xmlClusters = append(xmlClusters, convertToXMLCacheCluster(&clusters[i]))
	}

	writeXMLResponse(w, XMLDescribeCacheClustersResponse{
		Xmlns:         elasticacheXMLNS,
		CacheClusters: XMLCacheClusters{Items: xmlClusters},
		RequestID:     uuid.New().String(),
	})
}

// ModifyCacheCluster handles the ModifyCacheCluster action.
func (s *Service) ModifyCacheCluster(w http.ResponseWriter, r *http.Request) {
	var req ModifyCacheClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.CacheClusterID == "" {
		writeError(w, errInvalidParameterValue, "CacheClusterId is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.ModifyCacheCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLModifyCacheClusterResponse{
		Xmlns:        elasticacheXMLNS,
		CacheCluster: convertToXMLCacheCluster(cluster),
		RequestID:    uuid.New().String(),
	})
}

// CreateReplicationGroup handles the CreateReplicationGroup action.
func (s *Service) CreateReplicationGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateReplicationGroupInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ReplicationGroupID == "" {
		writeError(w, errInvalidParameterValue, "ReplicationGroupId is required", http.StatusBadRequest)

		return
	}

	if req.ReplicationGroupDescription == "" {
		writeError(w, errInvalidParameterValue, "ReplicationGroupDescription is required", http.StatusBadRequest)

		return
	}

	group, err := s.storage.CreateReplicationGroup(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateReplicationGroupResponse{
		Xmlns:            elasticacheXMLNS,
		ReplicationGroup: convertToXMLReplicationGroup(group),
		RequestID:        uuid.New().String(),
	})
}

// DeleteReplicationGroup handles the DeleteReplicationGroup action.
func (s *Service) DeleteReplicationGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteReplicationGroupInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ReplicationGroupID == "" {
		writeError(w, errInvalidParameterValue, "ReplicationGroupId is required", http.StatusBadRequest)

		return
	}

	group, err := s.storage.DeleteReplicationGroup(r.Context(), req.ReplicationGroupID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLDeleteReplicationGroupResponse{
		Xmlns:            elasticacheXMLNS,
		ReplicationGroup: convertToXMLReplicationGroup(group),
		RequestID:        uuid.New().String(),
	})
}

// DescribeReplicationGroups handles the DescribeReplicationGroups action.
func (s *Service) DescribeReplicationGroups(w http.ResponseWriter, r *http.Request) {
	var req DescribeReplicationGroupsInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	groups, err := s.storage.DescribeReplicationGroups(r.Context(), req.ReplicationGroupID)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlGroups := make([]XMLReplicationGroup, 0, len(groups))
	for i := range groups {
		xmlGroups = append(xmlGroups, convertToXMLReplicationGroup(&groups[i]))
	}

	writeXMLResponse(w, XMLDescribeReplicationGroupsResponse{
		Xmlns:             elasticacheXMLNS,
		ReplicationGroups: XMLReplicationGroups{Items: xmlGroups},
		RequestID:         uuid.New().String(),
	})
}

// Helper functions.

func extractAction(r *http.Request) string {
	target := r.Header.Get("X-Amz-Target")
	if target != "" {
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			return target[idx+1:]
		}
	}

	return r.URL.Query().Get("Action")
}

func writeXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Error: XMLError{
			Code:    code,
			Message: message,
		},
		RequestID: requestID,
	})
}

func handleError(w http.ResponseWriter, err error) {
	var ecErr *Error
	if errors.As(err, &ecErr) {
		status := http.StatusBadRequest
		if ecErr.Code == errCacheClusterNotFound || ecErr.Code == errReplicationGroupNotFound {
			status = http.StatusNotFound
		}

		writeError(w, ecErr.Code, ecErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)
}

// Conversion functions.

func convertToXMLCacheCluster(cluster *CacheCluster) XMLCacheCluster {
	var configEndpoint *XMLEndpoint
	if cluster.ConfigurationEndpoint != nil {
		configEndpoint = &XMLEndpoint{
			Address: cluster.ConfigurationEndpoint.Address,
			Port:    cluster.ConfigurationEndpoint.Port,
		}
	}

	cacheNodes := make([]XMLCacheNode, 0, len(cluster.CacheNodes))
	for _, node := range cluster.CacheNodes {
		cacheNodes = append(cacheNodes, convertToXMLCacheNode(&node))
	}

	securityGroups := make([]XMLSecurityGroupMembership, 0, len(cluster.SecurityGroups))
	for _, sg := range cluster.SecurityGroups {
		securityGroups = append(securityGroups, XMLSecurityGroupMembership(sg))
	}

	return XMLCacheCluster{
		CacheClusterID:             cluster.CacheClusterID,
		CacheClusterStatus:         cluster.CacheClusterStatus,
		CacheNodeType:              cluster.CacheNodeType,
		Engine:                     cluster.Engine,
		EngineVersion:              cluster.EngineVersion,
		NumCacheNodes:              cluster.NumCacheNodes,
		PreferredAvailabilityZone:  cluster.PreferredAvailabilityZone,
		CacheClusterCreateTime:     cluster.CacheClusterCreateTime.Format("2006-01-02T15:04:05.000Z"),
		PreferredMaintenanceWindow: cluster.PreferredMaintenanceWindow,
		CacheSubnetGroupName:       cluster.CacheSubnetGroupName,
		AutoMinorVersionUpgrade:    cluster.AutoMinorVersionUpgrade,
		SnapshotRetentionLimit:     cluster.SnapshotRetentionLimit,
		SnapshotWindow:             cluster.SnapshotWindow,
		ARN:                        cluster.ARN,
		CacheNodes:                 XMLCacheNodes{Items: cacheNodes},
		SecurityGroups:             XMLSecurityGroups{Items: securityGroups},
		ConfigurationEndpoint:      configEndpoint,
	}
}

func convertToXMLCacheNode(node *CacheNode) XMLCacheNode {
	var endpoint *XMLEndpoint
	if node.Endpoint != nil {
		endpoint = &XMLEndpoint{
			Address: node.Endpoint.Address,
			Port:    node.Endpoint.Port,
		}
	}

	return XMLCacheNode{
		CacheNodeID:              node.CacheNodeID,
		CacheNodeStatus:          node.CacheNodeStatus,
		CacheNodeCreateTime:      node.CacheNodeCreateTime.Format("2006-01-02T15:04:05.000Z"),
		Endpoint:                 endpoint,
		ParameterGroupStatus:     node.ParameterGroupStatus,
		CustomerAvailabilityZone: node.CustomerAvailabilityZone,
	}
}

func convertToXMLReplicationGroup(group *ReplicationGroup) XMLReplicationGroup {
	var configEndpoint *XMLEndpoint
	if group.ConfigurationEndpoint != nil {
		configEndpoint = &XMLEndpoint{
			Address: group.ConfigurationEndpoint.Address,
			Port:    group.ConfigurationEndpoint.Port,
		}
	}

	nodeGroups := make([]XMLNodeGroup, 0, len(group.NodeGroups))
	for _, ng := range group.NodeGroups {
		nodeGroups = append(nodeGroups, convertToXMLNodeGroup(&ng))
	}

	return XMLReplicationGroup{
		ReplicationGroupID:         group.ReplicationGroupID,
		Description:                group.Description,
		Status:                     group.Status,
		MemberClusters:             XMLMemberClusters{Items: group.MemberClusters},
		NodeGroups:                 XMLNodeGroups{Items: nodeGroups},
		AutomaticFailover:          group.AutomaticFailover,
		MultiAZ:                    group.MultiAZ,
		SnapshotRetentionLimit:     group.SnapshotRetentionLimit,
		SnapshotWindow:             group.SnapshotWindow,
		ClusterEnabled:             group.ClusterEnabled,
		CacheNodeType:              group.CacheNodeType,
		AuthTokenEnabled:           group.AuthTokenEnabled,
		TransitEncryptionEnabled:   group.TransitEncryptionEnabled,
		AtRestEncryptionEnabled:    group.AtRestEncryptionEnabled,
		ARN:                        group.ARN,
		ConfigurationEndpoint:      configEndpoint,
		ReplicationGroupCreateTime: group.ReplicationGroupCreateTime.Format("2006-01-02T15:04:05.000Z"),
		AutoMinorVersionUpgrade:    group.AutoMinorVersionUpgrade,
		PreferredMaintenanceWindow: group.PreferredMaintenanceWindow,
	}
}

func convertToXMLNodeGroup(ng *NodeGroup) XMLNodeGroup {
	var primaryEndpoint, readerEndpoint *XMLEndpoint
	if ng.PrimaryEndpoint != nil {
		primaryEndpoint = &XMLEndpoint{
			Address: ng.PrimaryEndpoint.Address,
			Port:    ng.PrimaryEndpoint.Port,
		}
	}

	if ng.ReaderEndpoint != nil {
		readerEndpoint = &XMLEndpoint{
			Address: ng.ReaderEndpoint.Address,
			Port:    ng.ReaderEndpoint.Port,
		}
	}

	members := make([]XMLNodeGroupMember, 0, len(ng.NodeGroupMembers))
	for _, m := range ng.NodeGroupMembers {
		members = append(members, convertToXMLNodeGroupMember(&m))
	}

	return XMLNodeGroup{
		NodeGroupID:      ng.NodeGroupID,
		Status:           ng.Status,
		PrimaryEndpoint:  primaryEndpoint,
		ReaderEndpoint:   readerEndpoint,
		NodeGroupMembers: XMLNodeGroupMembers{Items: members},
	}
}

func convertToXMLNodeGroupMember(m *NodeGroupMember) XMLNodeGroupMember {
	var readEndpoint *XMLEndpoint
	if m.ReadEndpoint != nil {
		readEndpoint = &XMLEndpoint{
			Address: m.ReadEndpoint.Address,
			Port:    m.ReadEndpoint.Port,
		}
	}

	return XMLNodeGroupMember{
		CacheClusterID:            m.CacheClusterID,
		CacheNodeID:               m.CacheNodeID,
		ReadEndpoint:              readEndpoint,
		PreferredAvailabilityZone: m.PreferredAvailabilityZone,
		CurrentRole:               m.CurrentRole,
	}
}

// XML response types.

// XMLCreateCacheClusterResponse is the XML response for CreateCacheCluster.
type XMLCreateCacheClusterResponse struct {
	XMLName      xml.Name        `xml:"CreateCacheClusterResponse"`
	Xmlns        string          `xml:"xmlns,attr"`
	CacheCluster XMLCacheCluster `xml:"CreateCacheClusterResult>CacheCluster"`
	RequestID    string          `xml:"ResponseMetadata>RequestId"`
}

// XMLDeleteCacheClusterResponse is the XML response for DeleteCacheCluster.
type XMLDeleteCacheClusterResponse struct {
	XMLName      xml.Name        `xml:"DeleteCacheClusterResponse"`
	Xmlns        string          `xml:"xmlns,attr"`
	CacheCluster XMLCacheCluster `xml:"DeleteCacheClusterResult>CacheCluster"`
	RequestID    string          `xml:"ResponseMetadata>RequestId"`
}

// XMLDescribeCacheClustersResponse is the XML response for DescribeCacheClusters.
type XMLDescribeCacheClustersResponse struct {
	XMLName       xml.Name         `xml:"DescribeCacheClustersResponse"`
	Xmlns         string           `xml:"xmlns,attr"`
	CacheClusters XMLCacheClusters `xml:"DescribeCacheClustersResult>CacheClusters"`
	RequestID     string           `xml:"ResponseMetadata>RequestId"`
}

// XMLModifyCacheClusterResponse is the XML response for ModifyCacheCluster.
type XMLModifyCacheClusterResponse struct {
	XMLName      xml.Name        `xml:"ModifyCacheClusterResponse"`
	Xmlns        string          `xml:"xmlns,attr"`
	CacheCluster XMLCacheCluster `xml:"ModifyCacheClusterResult>CacheCluster"`
	RequestID    string          `xml:"ResponseMetadata>RequestId"`
}

// XMLCreateReplicationGroupResponse is the XML response for CreateReplicationGroup.
type XMLCreateReplicationGroupResponse struct {
	XMLName          xml.Name            `xml:"CreateReplicationGroupResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ReplicationGroup XMLReplicationGroup `xml:"CreateReplicationGroupResult>ReplicationGroup"`
	RequestID        string              `xml:"ResponseMetadata>RequestId"`
}

// XMLDeleteReplicationGroupResponse is the XML response for DeleteReplicationGroup.
type XMLDeleteReplicationGroupResponse struct {
	XMLName          xml.Name            `xml:"DeleteReplicationGroupResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ReplicationGroup XMLReplicationGroup `xml:"DeleteReplicationGroupResult>ReplicationGroup"`
	RequestID        string              `xml:"ResponseMetadata>RequestId"`
}

// XMLDescribeReplicationGroupsResponse is the XML response for DescribeReplicationGroups.
type XMLDescribeReplicationGroupsResponse struct {
	XMLName           xml.Name             `xml:"DescribeReplicationGroupsResponse"`
	Xmlns             string               `xml:"xmlns,attr"`
	ReplicationGroups XMLReplicationGroups `xml:"DescribeReplicationGroupsResult>ReplicationGroups"`
	RequestID         string               `xml:"ResponseMetadata>RequestId"`
}

// XMLCacheCluster is the XML representation of a cache cluster.
type XMLCacheCluster struct {
	CacheClusterID             string            `xml:"CacheClusterId"`
	CacheClusterStatus         string            `xml:"CacheClusterStatus"`
	CacheNodeType              string            `xml:"CacheNodeType"`
	Engine                     string            `xml:"Engine"`
	EngineVersion              string            `xml:"EngineVersion,omitempty"`
	NumCacheNodes              int32             `xml:"NumCacheNodes"`
	PreferredAvailabilityZone  string            `xml:"PreferredAvailabilityZone,omitempty"`
	CacheClusterCreateTime     string            `xml:"CacheClusterCreateTime"`
	PreferredMaintenanceWindow string            `xml:"PreferredMaintenanceWindow,omitempty"`
	CacheSubnetGroupName       string            `xml:"CacheSubnetGroupName,omitempty"`
	AutoMinorVersionUpgrade    bool              `xml:"AutoMinorVersionUpgrade"`
	SnapshotRetentionLimit     int32             `xml:"SnapshotRetentionLimit"`
	SnapshotWindow             string            `xml:"SnapshotWindow,omitempty"`
	ARN                        string            `xml:"ARN"`
	CacheNodes                 XMLCacheNodes     `xml:"CacheNodes"`
	SecurityGroups             XMLSecurityGroups `xml:"SecurityGroups"`
	ConfigurationEndpoint      *XMLEndpoint      `xml:"ConfigurationEndpoint,omitempty"`
}

// XMLCacheClusters is a list of XML cache clusters.
type XMLCacheClusters struct {
	Items []XMLCacheCluster `xml:"CacheCluster"`
}

// XMLCacheNode is the XML representation of a cache node.
type XMLCacheNode struct {
	CacheNodeID              string       `xml:"CacheNodeId"`
	CacheNodeStatus          string       `xml:"CacheNodeStatus"`
	CacheNodeCreateTime      string       `xml:"CacheNodeCreateTime"`
	Endpoint                 *XMLEndpoint `xml:"Endpoint,omitempty"`
	ParameterGroupStatus     string       `xml:"ParameterGroupStatus,omitempty"`
	CustomerAvailabilityZone string       `xml:"CustomerAvailabilityZone,omitempty"`
}

// XMLCacheNodes is a list of XML cache nodes.
type XMLCacheNodes struct {
	Items []XMLCacheNode `xml:"CacheNode"`
}

// XMLEndpoint is the XML representation of an endpoint.
type XMLEndpoint struct {
	Address string `xml:"Address"`
	Port    int32  `xml:"Port"`
}

// XMLSecurityGroupMembership is the XML representation of a security group membership.
type XMLSecurityGroupMembership struct {
	SecurityGroupID string `xml:"SecurityGroupId"`
	Status          string `xml:"Status"`
}

// XMLSecurityGroups is a list of security group memberships.
type XMLSecurityGroups struct {
	Items []XMLSecurityGroupMembership `xml:"member"`
}

// XMLReplicationGroup is the XML representation of a replication group.
type XMLReplicationGroup struct {
	ReplicationGroupID         string            `xml:"ReplicationGroupId"`
	Description                string            `xml:"Description"`
	Status                     string            `xml:"Status"`
	MemberClusters             XMLMemberClusters `xml:"MemberClusters"`
	NodeGroups                 XMLNodeGroups     `xml:"NodeGroups"`
	AutomaticFailover          string            `xml:"AutomaticFailover"`
	MultiAZ                    string            `xml:"MultiAZ"`
	SnapshotRetentionLimit     int32             `xml:"SnapshotRetentionLimit"`
	SnapshotWindow             string            `xml:"SnapshotWindow,omitempty"`
	ClusterEnabled             bool              `xml:"ClusterEnabled"`
	CacheNodeType              string            `xml:"CacheNodeType,omitempty"`
	AuthTokenEnabled           bool              `xml:"AuthTokenEnabled"`
	TransitEncryptionEnabled   bool              `xml:"TransitEncryptionEnabled"`
	AtRestEncryptionEnabled    bool              `xml:"AtRestEncryptionEnabled"`
	ARN                        string            `xml:"ARN"`
	ConfigurationEndpoint      *XMLEndpoint      `xml:"ConfigurationEndpoint,omitempty"`
	ReplicationGroupCreateTime string            `xml:"ReplicationGroupCreateTime"`
	AutoMinorVersionUpgrade    bool              `xml:"AutoMinorVersionUpgrade"`
	PreferredMaintenanceWindow string            `xml:"PreferredMaintenanceWindow,omitempty"`
}

// XMLReplicationGroups is a list of XML replication groups.
type XMLReplicationGroups struct {
	Items []XMLReplicationGroup `xml:"ReplicationGroup"`
}

// XMLMemberClusters is a list of member cluster IDs.
type XMLMemberClusters struct {
	Items []string `xml:"ClusterId"`
}

// XMLNodeGroup is the XML representation of a node group.
type XMLNodeGroup struct {
	NodeGroupID      string              `xml:"NodeGroupId"`
	Status           string              `xml:"Status"`
	PrimaryEndpoint  *XMLEndpoint        `xml:"PrimaryEndpoint,omitempty"`
	ReaderEndpoint   *XMLEndpoint        `xml:"ReaderEndpoint,omitempty"`
	NodeGroupMembers XMLNodeGroupMembers `xml:"NodeGroupMembers"`
}

// XMLNodeGroups is a list of XML node groups.
type XMLNodeGroups struct {
	Items []XMLNodeGroup `xml:"NodeGroup"`
}

// XMLNodeGroupMember is the XML representation of a node group member.
type XMLNodeGroupMember struct {
	CacheClusterID            string       `xml:"CacheClusterId"`
	CacheNodeID               string       `xml:"CacheNodeId"`
	ReadEndpoint              *XMLEndpoint `xml:"ReadEndpoint,omitempty"`
	PreferredAvailabilityZone string       `xml:"PreferredAvailabilityZone,omitempty"`
	CurrentRole               string       `xml:"CurrentRole,omitempty"`
}

// XMLNodeGroupMembers is a list of XML node group members.
type XMLNodeGroupMembers struct {
	Items []XMLNodeGroupMember `xml:"NodeGroupMember"`
}

// XMLErrorResponse is the XML error response.
type XMLErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Error     XMLError `xml:"Error"`
	RequestID string   `xml:"RequestId"`
}

// XMLError is an XML error.
type XMLError struct {
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}
