// Package neptune implements the Neptune service handlers.
package neptune

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

const neptuneXMLNS = "http://rds.amazonaws.com/doc/2014-10-31/"

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)

	switch action {
	case "CreateDBCluster":
		s.CreateDBCluster(w, r)
	case "DeleteDBCluster":
		s.DeleteDBCluster(w, r)
	case "DescribeDBClusters":
		s.DescribeDBClusters(w, r)
	case "CreateDBInstance":
		s.CreateDBInstance(w, r)
	case "DeleteDBInstance":
		s.DeleteDBInstance(w, r)
	case "DescribeDBInstances":
		s.DescribeDBInstances(w, r)
	default:
		writeError(w, errInvalidParameterValue, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)
	}
}

// CreateDBCluster handles the CreateDBCluster action.
func (s *Service) CreateDBCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateDBClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBClusterIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBClusterIdentifier is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.CreateDBCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateDBClusterResponse{
		Xmlns:     neptuneXMLNS,
		DBCluster: convertToXMLDBCluster(cluster),
		RequestID: uuid.New().String(),
	})
}

// DeleteDBCluster handles the DeleteDBCluster action.
func (s *Service) DeleteDBCluster(w http.ResponseWriter, r *http.Request) {
	var req DeleteDBClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBClusterIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBClusterIdentifier is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.DeleteDBCluster(r.Context(), req.DBClusterIdentifier, req.SkipFinalSnapshot)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLDeleteDBClusterResponse{
		Xmlns:     neptuneXMLNS,
		DBCluster: convertToXMLDBCluster(cluster),
		RequestID: uuid.New().String(),
	})
}

// DescribeDBClusters handles the DescribeDBClusters action.
func (s *Service) DescribeDBClusters(w http.ResponseWriter, r *http.Request) {
	var req DescribeDBClustersInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	clusters, err := s.storage.DescribeDBClusters(r.Context(), req.DBClusterIdentifier)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlClusters := make([]XMLDBCluster, 0, len(clusters))
	for i := range clusters {
		xmlClusters = append(xmlClusters, convertToXMLDBCluster(&clusters[i]))
	}

	writeXMLResponse(w, XMLDescribeDBClustersResponse{
		Xmlns:      neptuneXMLNS,
		DBClusters: XMLDBClusters{Items: xmlClusters},
		RequestID:  uuid.New().String(),
	})
}

// CreateDBInstance handles the CreateDBInstance action.
func (s *Service) CreateDBInstance(w http.ResponseWriter, r *http.Request) {
	var req CreateDBInstanceInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBInstanceIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBInstanceIdentifier is required", http.StatusBadRequest)

		return
	}

	if req.DBInstanceClass == "" {
		writeError(w, errInvalidParameterValue, "DBInstanceClass is required", http.StatusBadRequest)

		return
	}

	instance, err := s.storage.CreateDBInstance(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateDBInstanceResponse{
		Xmlns:      neptuneXMLNS,
		DBInstance: convertToXMLDBInstance(instance),
		RequestID:  uuid.New().String(),
	})
}

// DeleteDBInstance handles the DeleteDBInstance action.
func (s *Service) DeleteDBInstance(w http.ResponseWriter, r *http.Request) {
	var req DeleteDBInstanceInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBInstanceIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBInstanceIdentifier is required", http.StatusBadRequest)

		return
	}

	instance, err := s.storage.DeleteDBInstance(r.Context(), req.DBInstanceIdentifier, req.SkipFinalSnapshot)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLDeleteDBInstanceResponse{
		Xmlns:      neptuneXMLNS,
		DBInstance: convertToXMLDBInstance(instance),
		RequestID:  uuid.New().String(),
	})
}

// DescribeDBInstances handles the DescribeDBInstances action.
func (s *Service) DescribeDBInstances(w http.ResponseWriter, r *http.Request) {
	var req DescribeDBInstancesInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	instances, err := s.storage.DescribeDBInstances(r.Context(), req.DBInstanceIdentifier)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlInstances := make([]XMLDBInstance, 0, len(instances))
	for i := range instances {
		xmlInstances = append(xmlInstances, convertToXMLDBInstance(&instances[i]))
	}

	writeXMLResponse(w, XMLDescribeDBInstancesResponse{
		Xmlns:       neptuneXMLNS,
		DBInstances: XMLDBInstances{Items: xmlInstances},
		RequestID:   uuid.New().String(),
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
	var neptuneErr *Error
	if errors.As(err, &neptuneErr) {
		status := http.StatusBadRequest
		if neptuneErr.Code == errDBClusterNotFound || neptuneErr.Code == errDBInstanceNotFound {
			status = http.StatusNotFound
		}

		writeError(w, neptuneErr.Code, neptuneErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)
}

// Conversion functions.

func convertToXMLDBCluster(cluster *DBCluster) XMLDBCluster {
	members := make([]XMLDBClusterMember, 0, len(cluster.DBClusterMembers))
	for _, m := range cluster.DBClusterMembers {
		//nolint:staticcheck // Cannot use type conversion due to different struct tags (json vs xml).
		members = append(members, XMLDBClusterMember{
			DBInstanceIdentifier:          m.DBInstanceIdentifier,
			IsClusterWriter:               m.IsClusterWriter,
			DBClusterParameterGroupStatus: m.DBClusterParameterGroupStatus,
		})
	}

	return XMLDBCluster{
		DBClusterIdentifier: cluster.DBClusterIdentifier,
		DBClusterArn:        cluster.DBClusterArn,
		Engine:              cluster.Engine,
		EngineVersion:       cluster.EngineVersion,
		Status:              cluster.Status,
		Endpoint:            cluster.Endpoint,
		ReaderEndpoint:      cluster.ReaderEndpoint,
		Port:                cluster.Port,
		ClusterCreateTime:   cluster.ClusterCreateTime.Format("2006-01-02T15:04:05.000Z"),
		DBClusterMembers:    XMLDBClusterMembers{Items: members},
	}
}

func convertToXMLDBInstance(inst *DBInstance) XMLDBInstance {
	var endpoint *XMLEndpoint
	if inst.Endpoint != nil {
		endpoint = &XMLEndpoint{
			Address: inst.Endpoint.Address,
			Port:    inst.Endpoint.Port,
		}
	}

	return XMLDBInstance{
		DBInstanceIdentifier: inst.DBInstanceIdentifier,
		DBInstanceArn:        inst.DBInstanceArn,
		DBInstanceClass:      inst.DBInstanceClass,
		Engine:               inst.Engine,
		EngineVersion:        inst.EngineVersion,
		DBInstanceStatus:     inst.DBInstanceStatus,
		Endpoint:             endpoint,
		DBClusterIdentifier:  inst.DBClusterIdentifier,
		InstanceCreateTime:   inst.InstanceCreateTime.Format("2006-01-02T15:04:05.000Z"),
	}
}

// XML response types.

// XMLCreateDBClusterResponse is the XML response for CreateDBCluster.
type XMLCreateDBClusterResponse struct {
	XMLName   xml.Name     `xml:"CreateDBClusterResponse"`
	Xmlns     string       `xml:"xmlns,attr"`
	DBCluster XMLDBCluster `xml:"CreateDBClusterResult>DBCluster"`
	RequestID string       `xml:"ResponseMetadata>RequestId"`
}

// XMLDeleteDBClusterResponse is the XML response for DeleteDBCluster.
type XMLDeleteDBClusterResponse struct {
	XMLName   xml.Name     `xml:"DeleteDBClusterResponse"`
	Xmlns     string       `xml:"xmlns,attr"`
	DBCluster XMLDBCluster `xml:"DeleteDBClusterResult>DBCluster"`
	RequestID string       `xml:"ResponseMetadata>RequestId"`
}

// XMLDescribeDBClustersResponse is the XML response for DescribeDBClusters.
type XMLDescribeDBClustersResponse struct {
	XMLName    xml.Name      `xml:"DescribeDBClustersResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	DBClusters XMLDBClusters `xml:"DescribeDBClustersResult>DBClusters"`
	RequestID  string        `xml:"ResponseMetadata>RequestId"`
}

// XMLCreateDBInstanceResponse is the XML response for CreateDBInstance.
type XMLCreateDBInstanceResponse struct {
	XMLName    xml.Name      `xml:"CreateDBInstanceResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	DBInstance XMLDBInstance `xml:"CreateDBInstanceResult>DBInstance"`
	RequestID  string        `xml:"ResponseMetadata>RequestId"`
}

// XMLDeleteDBInstanceResponse is the XML response for DeleteDBInstance.
type XMLDeleteDBInstanceResponse struct {
	XMLName    xml.Name      `xml:"DeleteDBInstanceResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	DBInstance XMLDBInstance `xml:"DeleteDBInstanceResult>DBInstance"`
	RequestID  string        `xml:"ResponseMetadata>RequestId"`
}

// XMLDescribeDBInstancesResponse is the XML response for DescribeDBInstances.
type XMLDescribeDBInstancesResponse struct {
	XMLName     xml.Name       `xml:"DescribeDBInstancesResponse"`
	Xmlns       string         `xml:"xmlns,attr"`
	DBInstances XMLDBInstances `xml:"DescribeDBInstancesResult>DBInstances"`
	RequestID   string         `xml:"ResponseMetadata>RequestId"`
}

// XMLDBCluster is the XML representation of a Neptune DB cluster.
type XMLDBCluster struct {
	DBClusterIdentifier string              `xml:"DBClusterIdentifier"`
	DBClusterArn        string              `xml:"DBClusterArn"`
	Engine              string              `xml:"Engine"`
	EngineVersion       string              `xml:"EngineVersion,omitempty"`
	Status              string              `xml:"Status"`
	Endpoint            string              `xml:"Endpoint"`
	ReaderEndpoint      string              `xml:"ReaderEndpoint"`
	Port                int32               `xml:"Port"`
	ClusterCreateTime   string              `xml:"ClusterCreateTime"`
	DBClusterMembers    XMLDBClusterMembers `xml:"DBClusterMembers"`
}

// XMLDBClusters is a list of XML DB clusters.
type XMLDBClusters struct {
	Items []XMLDBCluster `xml:"DBCluster"`
}

// XMLDBClusterMembers is a list of DB cluster members.
type XMLDBClusterMembers struct {
	Items []XMLDBClusterMember `xml:"DBClusterMember"`
}

// XMLDBClusterMember is the XML representation of a DB cluster member.
type XMLDBClusterMember struct {
	DBInstanceIdentifier          string `xml:"DBInstanceIdentifier"`
	IsClusterWriter               bool   `xml:"IsClusterWriter"`
	DBClusterParameterGroupStatus string `xml:"DBClusterParameterGroupStatus,omitempty"`
}

// XMLDBInstance is the XML representation of a Neptune DB instance.
type XMLDBInstance struct {
	DBInstanceIdentifier string       `xml:"DBInstanceIdentifier"`
	DBInstanceArn        string       `xml:"DBInstanceArn"`
	DBInstanceClass      string       `xml:"DBInstanceClass"`
	Engine               string       `xml:"Engine"`
	EngineVersion        string       `xml:"EngineVersion,omitempty"`
	DBInstanceStatus     string       `xml:"DBInstanceStatus"`
	Endpoint             *XMLEndpoint `xml:"Endpoint,omitempty"`
	DBClusterIdentifier  string       `xml:"DBClusterIdentifier,omitempty"`
	InstanceCreateTime   string       `xml:"InstanceCreateTime"`
}

// XMLDBInstances is a list of XML DB instances.
type XMLDBInstances struct {
	Items []XMLDBInstance `xml:"DBInstance"`
}

// XMLEndpoint is the XML representation of an endpoint.
type XMLEndpoint struct {
	Address string `xml:"Address"`
	Port    int32  `xml:"Port"`
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
