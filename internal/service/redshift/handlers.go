// Package redshift implements the Redshift service handlers.
package redshift

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

const redshiftXMLNS = "http://redshift.amazonaws.com/doc/2012-12-01/"

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)

	switch action {
	case "CreateCluster":
		s.CreateCluster(w, r)
	case "DeleteCluster":
		s.DeleteCluster(w, r)
	case "DescribeClusters":
		s.DescribeClusters(w, r)
	case "ModifyCluster":
		s.ModifyCluster(w, r)
	case "CreateClusterSnapshot":
		s.CreateClusterSnapshot(w, r)
	case "DeleteClusterSnapshot":
		s.DeleteClusterSnapshot(w, r)
	case "DescribeClusterSnapshots":
		s.DescribeClusterSnapshots(w, r)
	default:
		writeError(w, errInvalidParameterValue, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)
	}
}

// CreateCluster handles the CreateCluster action.
func (s *Service) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ClusterIdentifier == "" {
		writeError(w, errInvalidParameterValue, "ClusterIdentifier is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.CreateCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateClusterResponse{
		Xmlns:     redshiftXMLNS,
		Cluster:   convertToXMLCluster(cluster),
		RequestID: uuid.New().String(),
	})
}

// DeleteCluster handles the DeleteCluster action.
func (s *Service) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	var req DeleteClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ClusterIdentifier == "" {
		writeError(w, errInvalidParameterValue, "ClusterIdentifier is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.DeleteCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLDeleteClusterResponse{
		Xmlns:     redshiftXMLNS,
		Cluster:   convertToXMLCluster(cluster),
		RequestID: uuid.New().String(),
	})
}

// DescribeClusters handles the DescribeClusters action.
func (s *Service) DescribeClusters(w http.ResponseWriter, r *http.Request) {
	var req DescribeClustersInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	clusters, err := s.storage.DescribeClusters(r.Context(), req.ClusterIdentifier)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlClusters := make([]XMLCluster, 0, len(clusters))

	for i := range clusters {
		xmlClusters = append(xmlClusters, convertToXMLCluster(&clusters[i]))
	}

	writeXMLResponse(w, XMLDescribeClustersResponse{
		Xmlns:     redshiftXMLNS,
		Clusters:  XMLClusters{Items: xmlClusters},
		RequestID: uuid.New().String(),
	})
}

// ModifyCluster handles the ModifyCluster action.
func (s *Service) ModifyCluster(w http.ResponseWriter, r *http.Request) {
	var req ModifyClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ClusterIdentifier == "" {
		writeError(w, errInvalidParameterValue, "ClusterIdentifier is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.ModifyCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLModifyClusterResponse{
		Xmlns:     redshiftXMLNS,
		Cluster:   convertToXMLCluster(cluster),
		RequestID: uuid.New().String(),
	})
}

// CreateClusterSnapshot handles the CreateClusterSnapshot action.
func (s *Service) CreateClusterSnapshot(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterSnapshotInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.SnapshotIdentifier == "" {
		writeError(w, errInvalidParameterValue, "SnapshotIdentifier is required", http.StatusBadRequest)

		return
	}

	if req.ClusterIdentifier == "" {
		writeError(w, errInvalidParameterValue, "ClusterIdentifier is required", http.StatusBadRequest)

		return
	}

	snapshot, err := s.storage.CreateClusterSnapshot(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateClusterSnapshotResponse{
		Xmlns:     redshiftXMLNS,
		Snapshot:  convertToXMLSnapshot(snapshot),
		RequestID: uuid.New().String(),
	})
}

// DeleteClusterSnapshot handles the DeleteClusterSnapshot action.
func (s *Service) DeleteClusterSnapshot(w http.ResponseWriter, r *http.Request) {
	var req DeleteClusterSnapshotInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.SnapshotIdentifier == "" {
		writeError(w, errInvalidParameterValue, "SnapshotIdentifier is required", http.StatusBadRequest)

		return
	}

	snapshot, err := s.storage.DeleteClusterSnapshot(r.Context(), req.SnapshotIdentifier)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLDeleteClusterSnapshotResponse{
		Xmlns:     redshiftXMLNS,
		Snapshot:  convertToXMLSnapshot(snapshot),
		RequestID: uuid.New().String(),
	})
}

// DescribeClusterSnapshots handles the DescribeClusterSnapshots action.
func (s *Service) DescribeClusterSnapshots(w http.ResponseWriter, r *http.Request) {
	var req DescribeClusterSnapshotsInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	snapshots, err := s.storage.DescribeClusterSnapshots(r.Context(), req.ClusterIdentifier, req.SnapshotIdentifier)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlSnapshots := make([]XMLSnapshot, 0, len(snapshots))

	for i := range snapshots {
		xmlSnapshots = append(xmlSnapshots, convertToXMLSnapshot(&snapshots[i]))
	}

	writeXMLResponse(w, XMLDescribeClusterSnapshotsResponse{
		Xmlns:     redshiftXMLNS,
		Snapshots: XMLSnapshots{Items: xmlSnapshots},
		RequestID: uuid.New().String(),
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
	var redshiftErr *Error
	if errors.As(err, &redshiftErr) {
		status := http.StatusBadRequest
		if redshiftErr.Code == errClusterNotFound || redshiftErr.Code == errSnapshotNotFound {
			status = http.StatusNotFound
		}

		writeError(w, redshiftErr.Code, redshiftErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)
}

// Conversion functions.

func convertToXMLCluster(cluster *Cluster) XMLCluster {
	tags := make([]XMLTag, 0, len(cluster.Tags))

	for _, t := range cluster.Tags {
		//nolint:staticcheck // Cannot use type conversion due to different struct tags (json vs xml).
		tags = append(tags, XMLTag{
			Key:   t.Key,
			Value: t.Value,
		})
	}

	return XMLCluster{
		ClusterIdentifier:   cluster.ClusterIdentifier,
		ClusterNamespaceArn: cluster.ClusterNamespaceArn,
		NodeType:            cluster.NodeType,
		ClusterStatus:       cluster.ClusterStatus,
		MasterUsername:      cluster.MasterUsername,
		DBName:              cluster.DBName,
		Endpoint: XMLEndpoint{
			Address: cluster.Endpoint.Address,
			Port:    cluster.Endpoint.Port,
		},
		NumberOfNodes:     cluster.NumberOfNodes,
		ClusterCreateTime: cluster.ClusterCreateTime.Format("2006-01-02T15:04:05.000Z"),
		Tags:              XMLTagList{Tags: tags},
	}
}

func convertToXMLSnapshot(snapshot *ClusterSnapshot) XMLSnapshot {
	tags := make([]XMLTag, 0, len(snapshot.Tags))

	for _, t := range snapshot.Tags {
		//nolint:staticcheck // Cannot use type conversion due to different struct tags (json vs xml).
		tags = append(tags, XMLTag{
			Key:   t.Key,
			Value: t.Value,
		})
	}

	return XMLSnapshot{
		SnapshotIdentifier: snapshot.SnapshotIdentifier,
		ClusterIdentifier:  snapshot.ClusterIdentifier,
		SnapshotCreateTime: snapshot.SnapshotCreateTime.Format("2006-01-02T15:04:05.000Z"),
		Status:             snapshot.Status,
		Port:               snapshot.Port,
		NumberOfNodes:      snapshot.NumberOfNodes,
		DBName:             snapshot.DBName,
		MasterUsername:     snapshot.MasterUsername,
		Tags:               XMLTagList{Tags: tags},
	}
}

// XML response types.

// XMLCreateClusterResponse is the XML response for CreateCluster.
type XMLCreateClusterResponse struct {
	XMLName   xml.Name   `xml:"CreateClusterResponse"`
	Xmlns     string     `xml:"xmlns,attr"`
	Cluster   XMLCluster `xml:"CreateClusterResult>Cluster"`
	RequestID string     `xml:"ResponseMetadata>RequestId"`
}

// XMLDeleteClusterResponse is the XML response for DeleteCluster.
type XMLDeleteClusterResponse struct {
	XMLName   xml.Name   `xml:"DeleteClusterResponse"`
	Xmlns     string     `xml:"xmlns,attr"`
	Cluster   XMLCluster `xml:"DeleteClusterResult>Cluster"`
	RequestID string     `xml:"ResponseMetadata>RequestId"`
}

// XMLDescribeClustersResponse is the XML response for DescribeClusters.
type XMLDescribeClustersResponse struct {
	XMLName   xml.Name    `xml:"DescribeClustersResponse"`
	Xmlns     string      `xml:"xmlns,attr"`
	Clusters  XMLClusters `xml:"DescribeClustersResult>Clusters"`
	RequestID string      `xml:"ResponseMetadata>RequestId"`
}

// XMLModifyClusterResponse is the XML response for ModifyCluster.
type XMLModifyClusterResponse struct {
	XMLName   xml.Name   `xml:"ModifyClusterResponse"`
	Xmlns     string     `xml:"xmlns,attr"`
	Cluster   XMLCluster `xml:"ModifyClusterResult>Cluster"`
	RequestID string     `xml:"ResponseMetadata>RequestId"`
}

// XMLCreateClusterSnapshotResponse is the XML response for CreateClusterSnapshot.
type XMLCreateClusterSnapshotResponse struct {
	XMLName   xml.Name    `xml:"CreateClusterSnapshotResponse"`
	Xmlns     string      `xml:"xmlns,attr"`
	Snapshot  XMLSnapshot `xml:"CreateClusterSnapshotResult>Snapshot"`
	RequestID string      `xml:"ResponseMetadata>RequestId"`
}

// XMLDeleteClusterSnapshotResponse is the XML response for DeleteClusterSnapshot.
type XMLDeleteClusterSnapshotResponse struct {
	XMLName   xml.Name    `xml:"DeleteClusterSnapshotResponse"`
	Xmlns     string      `xml:"xmlns,attr"`
	Snapshot  XMLSnapshot `xml:"DeleteClusterSnapshotResult>Snapshot"`
	RequestID string      `xml:"ResponseMetadata>RequestId"`
}

// XMLDescribeClusterSnapshotsResponse is the XML response for DescribeClusterSnapshots.
type XMLDescribeClusterSnapshotsResponse struct {
	XMLName   xml.Name     `xml:"DescribeClusterSnapshotsResponse"`
	Xmlns     string       `xml:"xmlns,attr"`
	Snapshots XMLSnapshots `xml:"DescribeClusterSnapshotsResult>Snapshots"`
	RequestID string       `xml:"ResponseMetadata>RequestId"`
}

// XMLCluster is the XML representation of a Redshift cluster.
type XMLCluster struct {
	ClusterIdentifier   string      `xml:"ClusterIdentifier"`
	ClusterNamespaceArn string      `xml:"ClusterNamespaceArn"`
	NodeType            string      `xml:"NodeType"`
	ClusterStatus       string      `xml:"ClusterStatus"`
	MasterUsername      string      `xml:"MasterUsername"`
	DBName              string      `xml:"DBName"`
	Endpoint            XMLEndpoint `xml:"Endpoint"`
	NumberOfNodes       int32       `xml:"NumberOfNodes"`
	ClusterCreateTime   string      `xml:"ClusterCreateTime"`
	Tags                XMLTagList  `xml:"Tags"`
}

// XMLClusters is a list of XML clusters.
type XMLClusters struct {
	Items []XMLCluster `xml:"Cluster"`
}

// XMLSnapshot is the XML representation of a Redshift cluster snapshot.
type XMLSnapshot struct {
	SnapshotIdentifier string     `xml:"SnapshotIdentifier"`
	ClusterIdentifier  string     `xml:"ClusterIdentifier"`
	SnapshotCreateTime string     `xml:"SnapshotCreateTime"`
	Status             string     `xml:"Status"`
	Port               int32      `xml:"Port"`
	NumberOfNodes      int32      `xml:"NumberOfNodes"`
	DBName             string     `xml:"DBName"`
	MasterUsername     string     `xml:"MasterUsername"`
	Tags               XMLTagList `xml:"Tags"`
}

// XMLSnapshots is a list of XML snapshots.
type XMLSnapshots struct {
	Items []XMLSnapshot `xml:"Snapshot"`
}

// XMLEndpoint is the XML representation of an endpoint.
type XMLEndpoint struct {
	Address string `xml:"Address"`
	Port    int32  `xml:"Port"`
}

// XMLTag is the XML representation of a tag.
type XMLTag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// XMLTagList is a list of XML tags.
type XMLTagList struct {
	Tags []XMLTag `xml:"Tag"`
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
