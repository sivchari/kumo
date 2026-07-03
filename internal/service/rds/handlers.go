// Package rds implements the RDS service handlers.
package rds

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

const rdsXMLNS = "http://rds.amazonaws.com/doc/2014-10-31/"

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)

	switch action {
	case "CreateDBInstance":
		s.CreateDBInstance(w, r)
	case "DeleteDBInstance":
		s.DeleteDBInstance(w, r)
	case "DescribeDBInstances":
		s.DescribeDBInstances(w, r)
	case "ModifyDBInstance":
		s.ModifyDBInstance(w, r)
	case "StartDBInstance":
		s.StartDBInstance(w, r)
	case "StopDBInstance":
		s.StopDBInstance(w, r)
	case "CreateDBCluster":
		s.CreateDBCluster(w, r)
	case "DeleteDBCluster":
		s.DeleteDBCluster(w, r)
	case "DescribeDBClusters":
		s.DescribeDBClusters(w, r)
	case "ModifyDBCluster":
		s.ModifyDBCluster(w, r)
	case "CreateDBSnapshot":
		s.CreateDBSnapshot(w, r)
	case "DeleteDBSnapshot":
		s.DeleteDBSnapshot(w, r)
	default:
		writeError(w, errInvalidParameterValue, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)
	}
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

	if req.Engine == "" {
		writeError(w, errInvalidParameterValue, "Engine is required", http.StatusBadRequest)

		return
	}

	instance, err := s.storage.CreateDBInstance(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateDBInstanceResponse{
		Xmlns:      rdsXMLNS,
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
		Xmlns:      rdsXMLNS,
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
		Xmlns:       rdsXMLNS,
		DBInstances: XMLDBInstances{Items: xmlInstances},
		RequestID:   uuid.New().String(),
	})
}

// ModifyDBInstance handles the ModifyDBInstance action.
func (s *Service) ModifyDBInstance(w http.ResponseWriter, r *http.Request) {
	var req ModifyDBInstanceInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBInstanceIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBInstanceIdentifier is required", http.StatusBadRequest)

		return
	}

	instance, err := s.storage.ModifyDBInstance(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLModifyDBInstanceResponse{
		Xmlns:      rdsXMLNS,
		DBInstance: convertToXMLDBInstance(instance),
		RequestID:  uuid.New().String(),
	})
}

// StartDBInstance handles the StartDBInstance action.
func (s *Service) StartDBInstance(w http.ResponseWriter, r *http.Request) {
	var req StartDBInstanceInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBInstanceIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBInstanceIdentifier is required", http.StatusBadRequest)

		return
	}

	instance, err := s.storage.StartDBInstance(r.Context(), req.DBInstanceIdentifier)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLStartDBInstanceResponse{
		Xmlns:      rdsXMLNS,
		DBInstance: convertToXMLDBInstance(instance),
		RequestID:  uuid.New().String(),
	})
}

// StopDBInstance handles the StopDBInstance action.
func (s *Service) StopDBInstance(w http.ResponseWriter, r *http.Request) {
	var req StopDBInstanceInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBInstanceIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBInstanceIdentifier is required", http.StatusBadRequest)

		return
	}

	instance, err := s.storage.StopDBInstance(r.Context(), req.DBInstanceIdentifier)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLStopDBInstanceResponse{
		Xmlns:      rdsXMLNS,
		DBInstance: convertToXMLDBInstance(instance),
		RequestID:  uuid.New().String(),
	})
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

	if req.Engine == "" {
		writeError(w, errInvalidParameterValue, "Engine is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.CreateDBCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateDBClusterResponse{
		Xmlns:     rdsXMLNS,
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
		Xmlns:     rdsXMLNS,
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
		Xmlns:      rdsXMLNS,
		DBClusters: XMLDBClusters{Items: xmlClusters},
		RequestID:  uuid.New().String(),
	})
}

// ModifyDBCluster handles the ModifyDBCluster action.
func (s *Service) ModifyDBCluster(w http.ResponseWriter, r *http.Request) {
	var req ModifyDBClusterInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBClusterIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBClusterIdentifier is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.ModifyDBCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLModifyDBClusterResponse{
		Xmlns:     rdsXMLNS,
		DBCluster: convertToXMLDBCluster(cluster),
		RequestID: uuid.New().String(),
	})
}

// CreateDBSnapshot handles the CreateDBSnapshot action.
func (s *Service) CreateDBSnapshot(w http.ResponseWriter, r *http.Request) {
	var req CreateDBSnapshotInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBSnapshotIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBSnapshotIdentifier is required", http.StatusBadRequest)

		return
	}

	if req.DBInstanceIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBInstanceIdentifier is required", http.StatusBadRequest)

		return
	}

	snapshot, err := s.storage.CreateDBSnapshot(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateDBSnapshotResponse{
		Xmlns:      rdsXMLNS,
		DBSnapshot: convertToXMLDBSnapshot(snapshot),
		RequestID:  uuid.New().String(),
	})
}

// DeleteDBSnapshot handles the DeleteDBSnapshot action.
func (s *Service) DeleteDBSnapshot(w http.ResponseWriter, r *http.Request) {
	var req DeleteDBSnapshotInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.DBSnapshotIdentifier == "" {
		writeError(w, errInvalidParameterValue, "DBSnapshotIdentifier is required", http.StatusBadRequest)

		return
	}

	snapshot, err := s.storage.DeleteDBSnapshot(r.Context(), req.DBSnapshotIdentifier)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLDeleteDBSnapshotResponse{
		Xmlns:      rdsXMLNS,
		DBSnapshot: convertToXMLDBSnapshot(snapshot),
		RequestID:  uuid.New().String(),
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
	var rdsErr *Error
	if errors.As(err, &rdsErr) {
		status := http.StatusBadRequest
		if rdsErr.Code == errDBInstanceNotFound || rdsErr.Code == errDBClusterNotFound || rdsErr.Code == errDBSnapshotNotFound {
			status = http.StatusNotFound
		}

		writeError(w, rdsErr.Code, rdsErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)
}

// Conversion functions.

func convertToXMLDBInstance(inst *DBInstance) XMLDBInstance {
	var endpoint *XMLEndpoint
	if inst.Endpoint != nil {
		endpoint = &XMLEndpoint{
			Address:      inst.Endpoint.Address,
			Port:         inst.Endpoint.Port,
			HostedZoneID: inst.Endpoint.HostedZoneID,
		}
	}

	vpcSecurityGroups := make([]XMLVpcSecurityGroupMembership, 0, len(inst.VpcSecurityGroups))
	for _, sg := range inst.VpcSecurityGroups {
		vpcSecurityGroups = append(vpcSecurityGroups, XMLVpcSecurityGroupMembership(sg))
	}

	return XMLDBInstance{
		DBInstanceIdentifier:       inst.DBInstanceIdentifier,
		DBInstanceClass:            inst.DBInstanceClass,
		Engine:                     inst.Engine,
		EngineVersion:              inst.EngineVersion,
		DBInstanceStatus:           inst.DBInstanceStatus,
		MasterUsername:             inst.MasterUsername,
		DBName:                     inst.DBName,
		Endpoint:                   endpoint,
		AllocatedStorage:           inst.AllocatedStorage,
		InstanceCreateTime:         inst.InstanceCreateTime.Format("2006-01-02T15:04:05.000Z"),
		DBInstanceArn:              inst.DBInstanceArn,
		StorageType:                inst.StorageType,
		MultiAZ:                    inst.MultiAZ,
		AvailabilityZone:           inst.AvailabilityZone,
		BackupRetentionPeriod:      inst.BackupRetentionPeriod,
		PreferredBackupWindow:      inst.PreferredBackupWindow,
		PreferredMaintenanceWindow: inst.PreferredMaintenanceWindow,
		PubliclyAccessible:         inst.PubliclyAccessible,
		StorageEncrypted:           inst.StorageEncrypted,
		DeletionProtection:         inst.DeletionProtection,
		VpcSecurityGroups:          XMLVpcSecurityGroups{Items: vpcSecurityGroups},
	}
}

func convertToXMLDBCluster(cluster *DBCluster) XMLDBCluster {
	vpcSecurityGroups := make([]XMLVpcSecurityGroupMembership, 0, len(cluster.VpcSecurityGroups))
	for _, sg := range cluster.VpcSecurityGroups {
		vpcSecurityGroups = append(vpcSecurityGroups, XMLVpcSecurityGroupMembership(sg))
	}

	members := make([]XMLDBClusterMember, 0, len(cluster.DBClusterMembers))
	for _, m := range cluster.DBClusterMembers {
		members = append(members, XMLDBClusterMember{
			DBInstanceIdentifier: m.DBInstanceIdentifier,
			IsClusterWriter:      m.IsClusterWriter,
		})
	}

	return XMLDBCluster{
		DBClusterIdentifier: cluster.DBClusterIdentifier,
		DBClusterArn:        cluster.DBClusterArn,
		Engine:              cluster.Engine,
		EngineVersion:       cluster.EngineVersion,
		Status:              cluster.Status,
		MasterUsername:      cluster.MasterUsername,
		DatabaseName:        cluster.DatabaseName,
		Endpoint:            cluster.Endpoint,
		ReaderEndpoint:      cluster.ReaderEndpoint,
		Port:                cluster.Port,
		AllocatedStorage:    cluster.AllocatedStorage,
		ClusterCreateTime:   cluster.ClusterCreateTime.Format("2006-01-02T15:04:05.000Z"),
		MultiAZ:             cluster.MultiAZ,
		AvailabilityZones:   XMLAvailabilityZones{Items: cluster.AvailabilityZones},
		DBClusterMembers:    XMLDBClusterMembers{Items: members},
		VpcSecurityGroups:   XMLVpcSecurityGroups{Items: vpcSecurityGroups},
		StorageEncrypted:    cluster.StorageEncrypted,
		DeletionProtection:  cluster.DeletionProtection,
	}
}

func convertToXMLDBSnapshot(snapshot *DBSnapshot) XMLDBSnapshot {
	return XMLDBSnapshot{
		DBSnapshotIdentifier: snapshot.DBSnapshotIdentifier,
		DBSnapshotArn:        snapshot.DBSnapshotArn,
		DBInstanceIdentifier: snapshot.DBInstanceIdentifier,
		Engine:               snapshot.Engine,
		EngineVersion:        snapshot.EngineVersion,
		Status:               snapshot.Status,
		SnapshotType:         snapshot.SnapshotType,
		SnapshotCreateTime:   snapshot.SnapshotCreateTime.Format("2006-01-02T15:04:05.000Z"),
		AllocatedStorage:     snapshot.AllocatedStorage,
		Port:                 snapshot.Port,
		AvailabilityZone:     snapshot.AvailabilityZone,
		MasterUsername:       snapshot.MasterUsername,
		StorageType:          snapshot.StorageType,
		Encrypted:            snapshot.Encrypted,
	}
}

// XML response types.

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

// XMLModifyDBInstanceResponse is the XML response for ModifyDBInstance.
type XMLModifyDBInstanceResponse struct {
	XMLName    xml.Name      `xml:"ModifyDBInstanceResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	DBInstance XMLDBInstance `xml:"ModifyDBInstanceResult>DBInstance"`
	RequestID  string        `xml:"ResponseMetadata>RequestId"`
}

// XMLStartDBInstanceResponse is the XML response for StartDBInstance.
type XMLStartDBInstanceResponse struct {
	XMLName    xml.Name      `xml:"StartDBInstanceResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	DBInstance XMLDBInstance `xml:"StartDBInstanceResult>DBInstance"`
	RequestID  string        `xml:"ResponseMetadata>RequestId"`
}

// XMLStopDBInstanceResponse is the XML response for StopDBInstance.
type XMLStopDBInstanceResponse struct {
	XMLName    xml.Name      `xml:"StopDBInstanceResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	DBInstance XMLDBInstance `xml:"StopDBInstanceResult>DBInstance"`
	RequestID  string        `xml:"ResponseMetadata>RequestId"`
}

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

// XMLModifyDBClusterResponse is the XML response for ModifyDBCluster.
type XMLModifyDBClusterResponse struct {
	XMLName   xml.Name     `xml:"ModifyDBClusterResponse"`
	Xmlns     string       `xml:"xmlns,attr"`
	DBCluster XMLDBCluster `xml:"ModifyDBClusterResult>DBCluster"`
	RequestID string       `xml:"ResponseMetadata>RequestId"`
}

// XMLCreateDBSnapshotResponse is the XML response for CreateDBSnapshot.
type XMLCreateDBSnapshotResponse struct {
	XMLName    xml.Name      `xml:"CreateDBSnapshotResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	DBSnapshot XMLDBSnapshot `xml:"CreateDBSnapshotResult>DBSnapshot"`
	RequestID  string        `xml:"ResponseMetadata>RequestId"`
}

// XMLDeleteDBSnapshotResponse is the XML response for DeleteDBSnapshot.
type XMLDeleteDBSnapshotResponse struct {
	XMLName    xml.Name      `xml:"DeleteDBSnapshotResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	DBSnapshot XMLDBSnapshot `xml:"DeleteDBSnapshotResult>DBSnapshot"`
	RequestID  string        `xml:"ResponseMetadata>RequestId"`
}

// XMLDBInstance is the XML representation of a DB instance.
type XMLDBInstance struct {
	DBInstanceIdentifier       string               `xml:"DBInstanceIdentifier"`
	DBInstanceClass            string               `xml:"DBInstanceClass"`
	Engine                     string               `xml:"Engine"`
	EngineVersion              string               `xml:"EngineVersion,omitempty"`
	DBInstanceStatus           string               `xml:"DBInstanceStatus"`
	MasterUsername             string               `xml:"MasterUsername,omitempty"`
	DBName                     string               `xml:"DBName,omitempty"`
	Endpoint                   *XMLEndpoint         `xml:"Endpoint,omitempty"`
	AllocatedStorage           int32                `xml:"AllocatedStorage"`
	InstanceCreateTime         string               `xml:"InstanceCreateTime"`
	DBInstanceArn              string               `xml:"DBInstanceArn"`
	StorageType                string               `xml:"StorageType,omitempty"`
	MultiAZ                    bool                 `xml:"MultiAZ"`
	AvailabilityZone           string               `xml:"AvailabilityZone,omitempty"`
	BackupRetentionPeriod      int32                `xml:"BackupRetentionPeriod"`
	PreferredBackupWindow      string               `xml:"PreferredBackupWindow,omitempty"`
	PreferredMaintenanceWindow string               `xml:"PreferredMaintenanceWindow,omitempty"`
	PubliclyAccessible         bool                 `xml:"PubliclyAccessible"`
	StorageEncrypted           bool                 `xml:"StorageEncrypted"`
	DeletionProtection         bool                 `xml:"DeletionProtection"`
	VpcSecurityGroups          XMLVpcSecurityGroups `xml:"VpcSecurityGroups"`
}

// XMLDBInstances is a list of XML DB instances.
type XMLDBInstances struct {
	Items []XMLDBInstance `xml:"DBInstance"`
}

// XMLEndpoint is the XML representation of an endpoint.
type XMLEndpoint struct {
	Address      string `xml:"Address"`
	Port         int32  `xml:"Port"`
	HostedZoneID string `xml:"HostedZoneId,omitempty"`
}

// XMLVpcSecurityGroups is a list of VPC security group memberships.
type XMLVpcSecurityGroups struct {
	Items []XMLVpcSecurityGroupMembership `xml:"VpcSecurityGroupMembership"`
}

// XMLVpcSecurityGroupMembership is the XML representation of a VPC security group membership.
type XMLVpcSecurityGroupMembership struct {
	VpcSecurityGroupID string `xml:"VpcSecurityGroupId"`
	Status             string `xml:"Status"`
}

// XMLDBCluster is the XML representation of a DB cluster.
type XMLDBCluster struct {
	DBClusterIdentifier string               `xml:"DBClusterIdentifier"`
	DBClusterArn        string               `xml:"DBClusterArn"`
	Engine              string               `xml:"Engine"`
	EngineVersion       string               `xml:"EngineVersion,omitempty"`
	Status              string               `xml:"Status"`
	MasterUsername      string               `xml:"MasterUsername,omitempty"`
	DatabaseName        string               `xml:"DatabaseName,omitempty"`
	Endpoint            string               `xml:"Endpoint"`
	ReaderEndpoint      string               `xml:"ReaderEndpoint"`
	Port                int32                `xml:"Port"`
	AllocatedStorage    int32                `xml:"AllocatedStorage"`
	ClusterCreateTime   string               `xml:"ClusterCreateTime"`
	MultiAZ             bool                 `xml:"MultiAZ"`
	AvailabilityZones   XMLAvailabilityZones `xml:"AvailabilityZones"`
	DBClusterMembers    XMLDBClusterMembers  `xml:"DBClusterMembers"`
	VpcSecurityGroups   XMLVpcSecurityGroups `xml:"VpcSecurityGroups"`
	StorageEncrypted    bool                 `xml:"StorageEncrypted"`
	DeletionProtection  bool                 `xml:"DeletionProtection"`
}

// XMLDBClusters is a list of XML DB clusters.
type XMLDBClusters struct {
	Items []XMLDBCluster `xml:"DBCluster"`
}

// XMLAvailabilityZones is a list of availability zones.
type XMLAvailabilityZones struct {
	Items []string `xml:"AvailabilityZone"`
}

// XMLDBClusterMembers is a list of DB cluster members.
type XMLDBClusterMembers struct {
	Items []XMLDBClusterMember `xml:"DBClusterMember"`
}

// XMLDBClusterMember is the XML representation of a DB cluster member.
type XMLDBClusterMember struct {
	DBInstanceIdentifier string `xml:"DBInstanceIdentifier"`
	IsClusterWriter      bool   `xml:"IsClusterWriter"`
}

// XMLDBSnapshot is the XML representation of a DB snapshot.
type XMLDBSnapshot struct {
	DBSnapshotIdentifier string `xml:"DBSnapshotIdentifier"`
	DBSnapshotArn        string `xml:"DBSnapshotArn"`
	DBInstanceIdentifier string `xml:"DBInstanceIdentifier"`
	Engine               string `xml:"Engine"`
	EngineVersion        string `xml:"EngineVersion,omitempty"`
	Status               string `xml:"Status"`
	SnapshotType         string `xml:"SnapshotType"`
	SnapshotCreateTime   string `xml:"SnapshotCreateTime"`
	AllocatedStorage     int32  `xml:"AllocatedStorage"`
	Port                 int32  `xml:"Port"`
	AvailabilityZone     string `xml:"AvailabilityZone,omitempty"`
	MasterUsername       string `xml:"MasterUsername,omitempty"`
	StorageType          string `xml:"StorageType,omitempty"`
	Encrypted            bool   `xml:"Encrypted"`
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
