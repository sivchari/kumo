// Package ec2 provides EC2 service emulation for kumo.
package ec2

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const ec2XMLNS = "http://ec2.amazonaws.com/doc/2016-11-15/"

// Error codes for EC2.
const (
	errInvalidParameter = "InvalidParameterValue"
	errInternalError    = "InternalError"
	errInvalidAction    = "InvalidAction"
)

// RunInstances handles the RunInstances action.
func (s *Service) RunInstances(w http.ResponseWriter, r *http.Request) {
	var req RunInstancesRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ImageID == "" {
		writeError(w, errInvalidParameter, "ImageId is required", http.StatusBadRequest)

		return
	}

	instances, reservationID, err := s.storage.RunInstances(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlInstances := make([]XMLInstance, 0, len(instances))
	for _, inst := range instances {
		xmlInstances = append(xmlInstances, convertToXMLInstance(inst))
	}

	writeEC2XMLResponse(w, XMLRunInstancesResponse{
		Xmlns:         ec2XMLNS,
		RequestID:     uuid.New().String(),
		ReservationID: reservationID,
		OwnerID:       defaultAccountID,
		InstancesSet:  XMLInstancesSet{Items: xmlInstances},
	})
}

// TerminateInstances handles the TerminateInstances action.
func (s *Service) TerminateInstances(w http.ResponseWriter, r *http.Request) {
	var req TerminateInstancesRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.InstanceIDs) == 0 {
		writeError(w, errInvalidParameter, "InstanceIds is required", http.StatusBadRequest)

		return
	}

	changes, err := s.storage.TerminateInstances(r.Context(), req.InstanceIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLTerminateInstancesResponse{
		Xmlns:        ec2XMLNS,
		RequestID:    uuid.New().String(),
		InstancesSet: convertToXMLInstanceStateChangeSet(changes),
	})
}

// DescribeInstances handles the DescribeInstances action.
func (s *Service) DescribeInstances(w http.ResponseWriter, r *http.Request) {
	var req DescribeInstancesRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	reservations, err := s.storage.DescribeInstances(r.Context(), req.InstanceIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlReservations := make([]XMLReservation, 0, len(reservations))

	for _, res := range reservations {
		xmlInstances := make([]XMLInstance, 0, len(res.Instances))
		for _, inst := range res.Instances {
			xmlInstances = append(xmlInstances, convertToXMLInstance(inst))
		}

		xmlReservations = append(xmlReservations, XMLReservation{
			ReservationID: res.ReservationID,
			OwnerID:       res.OwnerID,
			InstancesSet:  XMLInstancesSet{Items: xmlInstances},
		})
	}

	writeEC2XMLResponse(w, XMLDescribeInstancesResponse{
		Xmlns:          ec2XMLNS,
		RequestID:      uuid.New().String(),
		ReservationSet: XMLReservationSet{Items: xmlReservations},
	})
}

// StartInstances handles the StartInstances action.
func (s *Service) StartInstances(w http.ResponseWriter, r *http.Request) {
	var req StartInstancesRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.InstanceIDs) == 0 {
		writeError(w, errInvalidParameter, "InstanceIds is required", http.StatusBadRequest)

		return
	}

	changes, err := s.storage.StartInstances(r.Context(), req.InstanceIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLStartInstancesResponse{
		Xmlns:        ec2XMLNS,
		RequestID:    uuid.New().String(),
		InstancesSet: convertToXMLInstanceStateChangeSet(changes),
	})
}

// StopInstances handles the StopInstances action.
func (s *Service) StopInstances(w http.ResponseWriter, r *http.Request) {
	var req StopInstancesRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.InstanceIDs) == 0 {
		writeError(w, errInvalidParameter, "InstanceIds is required", http.StatusBadRequest)

		return
	}

	changes, err := s.storage.StopInstances(r.Context(), req.InstanceIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLStopInstancesResponse{
		Xmlns:        ec2XMLNS,
		RequestID:    uuid.New().String(),
		InstancesSet: convertToXMLInstanceStateChangeSet(changes),
	})
}

// CreateSecurityGroup handles the CreateSecurityGroup action.
func (s *Service) CreateSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateSecurityGroupRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.GroupName == "" {
		writeError(w, errInvalidParameter, "GroupName is required", http.StatusBadRequest)

		return
	}

	if req.GroupDescription == "" {
		writeError(w, errInvalidParameter, "GroupDescription is required", http.StatusBadRequest)

		return
	}

	sg, err := s.storage.CreateSecurityGroup(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	applyTagsOnCreate(r, s.storage, sg.GroupID, "security-group")

	writeEC2XMLResponse(w, XMLCreateSecurityGroupResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
		GroupID:   sg.GroupID,
	})
}

// DeleteSecurityGroup handles the DeleteSecurityGroup action.
func (s *Service) DeleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteSecurityGroupRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.GroupID == "" && req.GroupName == "" {
		writeError(w, errInvalidParameter, "GroupId or GroupName is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteSecurityGroup(r.Context(), req.GroupID, req.GroupName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLDeleteSecurityGroupResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// AuthorizeSecurityGroupIngress handles the AuthorizeSecurityGroupIngress action.
func (s *Service) AuthorizeSecurityGroupIngress(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeSecurityGroupIngressRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.GroupID == "" && req.GroupName == "" {
		writeError(w, errInvalidParameter, "GroupId or GroupName is required", http.StatusBadRequest)

		return
	}

	// The form-to-JSON converter doesn't expand
	// IpPermissions.N.{IpProtocol,FromPort,ToPort,IpRanges.M.CidrIp}
	// into a slice — the dotted keys land at the top level and never
	// unmarshal into req.IPPermissions. Re-parse the form directly so
	// the ingress rules actually reach storage. (Same bug class as the
	// RegisterTargets / CreateListener / CreateRule fixes.)
	if err := r.ParseForm(); err == nil {
		if perms := parseIPPermissionsFromForm(r.Form); len(perms) > 0 {
			req.IPPermissions = perms
		}
	}

	err := s.storage.AuthorizeSecurityGroupIngress(r.Context(), req.GroupID, req.GroupName, req.IPPermissions)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLAuthorizeSecurityGroupIngressResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// AuthorizeSecurityGroupEgress handles the AuthorizeSecurityGroupEgress action.
func (s *Service) AuthorizeSecurityGroupEgress(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeSecurityGroupEgressRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.GroupID == "" {
		writeError(w, errInvalidParameter, "GroupId is required", http.StatusBadRequest)

		return
	}

	// Same form-converter limitation as AuthorizeSecurityGroupIngress
	// above; re-parse so egress rules survive into storage.
	if err := r.ParseForm(); err == nil {
		if perms := parseIPPermissionsFromForm(r.Form); len(perms) > 0 {
			req.IPPermissions = perms
		}
	}

	err := s.storage.AuthorizeSecurityGroupEgress(r.Context(), req.GroupID, req.IPPermissions)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLAuthorizeSecurityGroupEgressResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// CreateKeyPair handles the CreateKeyPair action.
func (s *Service) CreateKeyPair(w http.ResponseWriter, r *http.Request) {
	var req CreateKeyPairRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.KeyName == "" {
		writeError(w, errInvalidParameter, "KeyName is required", http.StatusBadRequest)

		return
	}

	kp, err := s.storage.CreateKeyPair(r.Context(), req.KeyName, req.KeyType)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLCreateKeyPairResponse{
		Xmlns:          ec2XMLNS,
		RequestID:      uuid.New().String(),
		KeyName:        kp.KeyName,
		KeyFingerprint: kp.KeyFingerprint,
		KeyMaterial:    kp.KeyMaterial,
		KeyPairID:      kp.KeyPairID,
	})
}

// DeleteKeyPair handles the DeleteKeyPair action.
func (s *Service) DeleteKeyPair(w http.ResponseWriter, r *http.Request) {
	var req DeleteKeyPairRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.KeyName == "" && req.KeyPairID == "" {
		writeError(w, errInvalidParameter, "KeyName or KeyPairId is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteKeyPair(r.Context(), req.KeyName, req.KeyPairID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLDeleteKeyPairResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// DescribeKeyPairs handles the DescribeKeyPairs action.
func (s *Service) DescribeKeyPairs(w http.ResponseWriter, r *http.Request) {
	var req DescribeKeyPairsRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	keyPairs, err := s.storage.DescribeKeyPairs(r.Context(), req.KeyNames, req.KeyPairIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlKeyPairs := make([]XMLKeyPairInfo, 0, len(keyPairs))
	for _, kp := range keyPairs {
		xmlKeyPairs = append(xmlKeyPairs, XMLKeyPairInfo{
			KeyName:        kp.KeyName,
			KeyFingerprint: kp.KeyFingerprint,
			KeyPairID:      kp.KeyPairID,
		})
	}

	writeEC2XMLResponse(w, XMLDescribeKeyPairsResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		KeySet:    XMLKeyPairSet{Items: xmlKeyPairs},
	})
}

// CreateVpc handles the CreateVpc action.
func (s *Service) CreateVpc(w http.ResponseWriter, r *http.Request) {
	var req CreateVpcRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.CidrBlock == "" {
		writeError(w, errInvalidParameter, "CidrBlock is required", http.StatusBadRequest)

		return
	}

	vpc, err := s.storage.CreateVpc(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	applyTagsOnCreate(r, s.storage, vpc.VpcID, "vpc")

	writeEC2XMLResponse(w, XMLCreateVpcResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Vpc:       convertToXMLVpc(vpc),
	})
}

// applyTagsOnCreate copies TagSpecifications from the form (if any) onto the
// just-created resource via the storage tag API. Storage.CreateTags upserts in
// place, and the existing Storage.Create* methods return the storage-owned
// pointer, so the caller's resource value reflects the new tags by the time
// this returns — no further mutation needed. resourceType matches the AWS
// TagSpecifications.ResourceType values: "vpc", "subnet", "internet-gateway",
// "route-table", "security-group".
func applyTagsOnCreate(r *http.Request, storage Storage, resourceID, resourceType string) {
	if err := r.ParseForm(); err != nil {
		return
	}

	tags := parseTagSpecificationsForResourceType(r.Form, resourceType)
	if len(tags) == 0 {
		return
	}

	_ = storage.CreateTags(r.Context(), []string{resourceID}, tags)
}

// DeleteVpc handles the DeleteVpc action.
func (s *Service) DeleteVpc(w http.ResponseWriter, r *http.Request) {
	var req DeleteVpcRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.VpcID == "" {
		writeError(w, errInvalidParameter, "VpcId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteVpc(r.Context(), req.VpcID); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLDeleteVpcResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// DescribeVpcs handles the DescribeVpcs action.
func (s *Service) DescribeVpcs(w http.ResponseWriter, r *http.Request) {
	var req DescribeVpcsRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	vpcs, err := s.storage.DescribeVpcs(r.Context(), req.VpcIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlVpcs := make([]XMLVpc, 0, len(vpcs))
	for _, vpc := range vpcs {
		xmlVpcs = append(xmlVpcs, convertToXMLVpc(vpc))
	}

	writeEC2XMLResponse(w, XMLDescribeVpcsResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		VpcSet:    XMLVpcSet{Items: xmlVpcs},
	})
}

// CreateSubnet handles the CreateSubnet action.
func (s *Service) CreateSubnet(w http.ResponseWriter, r *http.Request) {
	var req CreateSubnetRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.VpcID == "" {
		writeError(w, errInvalidParameter, "VpcId is required", http.StatusBadRequest)

		return
	}

	if req.CidrBlock == "" {
		writeError(w, errInvalidParameter, "CidrBlock is required", http.StatusBadRequest)

		return
	}

	subnet, err := s.storage.CreateSubnet(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	applyTagsOnCreate(r, s.storage, subnet.SubnetID, "subnet")

	writeEC2XMLResponse(w, XMLCreateSubnetResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Subnet:    convertToXMLSubnet(subnet),
	})
}

// DeleteSubnet handles the DeleteSubnet action.
func (s *Service) DeleteSubnet(w http.ResponseWriter, r *http.Request) {
	var req DeleteSubnetRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.SubnetID == "" {
		writeError(w, errInvalidParameter, "SubnetId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteSubnet(r.Context(), req.SubnetID); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLDeleteSubnetResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// DescribeSubnets handles the DescribeSubnets action.
func (s *Service) DescribeSubnets(w http.ResponseWriter, r *http.Request) {
	var req DescribeSubnetsRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	subnets, err := s.storage.DescribeSubnets(r.Context(), req.SubnetIDs, nil)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlSubnets := make([]XMLSubnet, 0, len(subnets))
	for _, subnet := range subnets {
		xmlSubnets = append(xmlSubnets, convertToXMLSubnet(subnet))
	}

	writeEC2XMLResponse(w, XMLDescribeSubnetsResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		SubnetSet: XMLSubnetSet{Items: xmlSubnets},
	})
}

// CreateInternetGateway handles the CreateInternetGateway action.
func (s *Service) CreateInternetGateway(w http.ResponseWriter, r *http.Request) {
	var req CreateInternetGatewayRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	igw, err := s.storage.CreateInternetGateway(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	applyTagsOnCreate(r, s.storage, igw.InternetGatewayID, "internet-gateway")

	writeEC2XMLResponse(w, XMLCreateInternetGatewayResponse{
		Xmlns:           ec2XMLNS,
		RequestID:       uuid.New().String(),
		InternetGateway: convertToXMLInternetGateway(igw),
	})
}

// AttachInternetGateway handles the AttachInternetGateway action.
func (s *Service) AttachInternetGateway(w http.ResponseWriter, r *http.Request) {
	var req AttachInternetGatewayRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.InternetGatewayID == "" {
		writeError(w, errInvalidParameter, "InternetGatewayId is required", http.StatusBadRequest)

		return
	}

	if req.VpcID == "" {
		writeError(w, errInvalidParameter, "VpcId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.AttachInternetGateway(r.Context(), req.InternetGatewayID, req.VpcID); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLAttachInternetGatewayResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// DescribeInternetGateways handles the DescribeInternetGateways action.
func (s *Service) DescribeInternetGateways(w http.ResponseWriter, r *http.Request) {
	var req DescribeInternetGatewaysRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	igws, err := s.storage.DescribeInternetGateways(r.Context(), req.InternetGatewayIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlIgws := make([]XMLInternetGateway, 0, len(igws))
	for _, igw := range igws {
		xmlIgws = append(xmlIgws, convertToXMLInternetGateway(igw))
	}

	writeEC2XMLResponse(w, XMLDescribeInternetGatewaysResponse{
		Xmlns:              ec2XMLNS,
		RequestID:          uuid.New().String(),
		InternetGatewaySet: XMLInternetGatewaySet{Items: xmlIgws},
	})
}

// CreateRouteTable handles the CreateRouteTable action.
func (s *Service) CreateRouteTable(w http.ResponseWriter, r *http.Request) {
	var req CreateRouteTableRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.VpcID == "" {
		writeError(w, errInvalidParameter, "VpcId is required", http.StatusBadRequest)

		return
	}

	rt, err := s.storage.CreateRouteTable(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	applyTagsOnCreate(r, s.storage, rt.RouteTableID, "route-table")

	writeEC2XMLResponse(w, XMLCreateRouteTableResponse{
		Xmlns:      ec2XMLNS,
		RequestID:  uuid.New().String(),
		RouteTable: convertToXMLRouteTable(rt),
	})
}

// CreateRoute handles the CreateRoute action.
func (s *Service) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var req CreateRouteRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.RouteTableID == "" {
		writeError(w, errInvalidParameter, "RouteTableId is required", http.StatusBadRequest)

		return
	}

	if req.DestinationCidrBlock == "" {
		writeError(w, errInvalidParameter, "DestinationCidrBlock is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateRoute(r.Context(), &req); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLCreateRouteResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// AssociateRouteTable handles the AssociateRouteTable action.
func (s *Service) AssociateRouteTable(w http.ResponseWriter, r *http.Request) {
	var req AssociateRouteTableRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.RouteTableID == "" {
		writeError(w, errInvalidParameter, "RouteTableId is required", http.StatusBadRequest)

		return
	}

	if req.SubnetID == "" {
		writeError(w, errInvalidParameter, "SubnetId is required", http.StatusBadRequest)

		return
	}

	associationID, err := s.storage.AssociateRouteTable(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLAssociateRouteTableResponse{
		Xmlns:         ec2XMLNS,
		RequestID:     uuid.New().String(),
		AssociationID: associationID,
	})
}

// DescribeRouteTables handles the DescribeRouteTables action.
func (s *Service) DescribeRouteTables(w http.ResponseWriter, r *http.Request) {
	var req DescribeRouteTablesRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	rts, err := s.storage.DescribeRouteTables(r.Context(), req.RouteTableIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlRts := make([]XMLRouteTable, 0, len(rts))
	for _, rt := range rts {
		xmlRts = append(xmlRts, convertToXMLRouteTable(rt))
	}

	writeEC2XMLResponse(w, XMLDescribeRouteTablesResponse{
		Xmlns:         ec2XMLNS,
		RequestID:     uuid.New().String(),
		RouteTableSet: XMLRouteTableSet{Items: xmlRts},
	})
}

// CreateNatGateway handles the CreateNatGateway action.
func (s *Service) CreateNatGateway(w http.ResponseWriter, r *http.Request) {
	var req CreateNatGatewayRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.SubnetID == "" {
		writeError(w, errInvalidParameter, "SubnetId is required", http.StatusBadRequest)

		return
	}

	natgw, err := s.storage.CreateNatGateway(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLCreateNatGatewayResponse{
		Xmlns:      ec2XMLNS,
		RequestID:  uuid.New().String(),
		NatGateway: convertToXMLNatGateway(natgw),
	})
}

// DescribeNatGateways handles the DescribeNatGateways action.
func (s *Service) DescribeNatGateways(w http.ResponseWriter, r *http.Request) {
	var req DescribeNatGatewaysRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	natgws, err := s.storage.DescribeNatGateways(r.Context(), req.NatGatewayIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlNatgws := make([]XMLNatGateway, 0, len(natgws))
	for _, natgw := range natgws {
		xmlNatgws = append(xmlNatgws, convertToXMLNatGateway(natgw))
	}

	writeEC2XMLResponse(w, XMLDescribeNatGatewaysResponse{
		Xmlns:         ec2XMLNS,
		RequestID:     uuid.New().String(),
		NatGatewaySet: XMLNatGatewaySet{Items: xmlNatgws},
	})
}

// CreateTags handles the CreateTags action.
func (s *Service) CreateTags(w http.ResponseWriter, r *http.Request) {
	resources, tags, err := readTagRequestForm(r)
	if err != nil {
		writeError(w, errInvalidParameter, err.Error(), http.StatusBadRequest)

		return
	}

	if len(resources) == 0 {
		writeError(w, errInvalidParameter, "ResourceId is required", http.StatusBadRequest)

		return
	}

	if len(tags) == 0 {
		writeError(w, errInvalidParameter, "Tag is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateTags(r.Context(), resources, tags); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLCreateTagsResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// DeleteTags handles the DeleteTags action.
func (s *Service) DeleteTags(w http.ResponseWriter, r *http.Request) {
	resources, tags, err := readTagRequestForm(r)
	if err != nil {
		writeError(w, errInvalidParameter, err.Error(), http.StatusBadRequest)

		return
	}

	if len(resources) == 0 {
		writeError(w, errInvalidParameter, "ResourceId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteTags(r.Context(), resources, tags); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLDeleteTagsResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// DescribeTags handles the DescribeTags action.
func (s *Service) DescribeTags(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	filters := parseFiltersFromForm(r.Form)

	descriptions, err := s.storage.DescribeTags(r.Context(), filters)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]XMLTagDescription, 0, len(descriptions))
	for _, d := range descriptions {
		items = append(items, XMLTagDescription(d))
	}

	writeEC2XMLResponse(w, XMLDescribeTagsResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		TagSet:    XMLTagDescriptionSet{Items: items},
	})
}

// ModifyVpcAttribute handles the ModifyVpcAttribute action. AWS modifies one
// attribute per call; the handler updates only the fields the request supplies.
func (s *Service) ModifyVpcAttribute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	vpcID := r.Form.Get("VpcId")
	if vpcID == "" {
		writeError(w, errInvalidParameter, "VpcId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.ModifyVpcAttribute(r.Context(), vpcID, vpcAttributeUpdates(r.Form)); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, struct {
		XMLName   xml.Name `xml:"ModifyVpcAttributeResponse"`
		Xmlns     string   `xml:"xmlns,attr"`
		RequestID string   `xml:"requestId"`
		Return    bool     `xml:"return"`
	}{Xmlns: ec2XMLNS, RequestID: uuid.New().String(), Return: true})
}

// DescribeVpcAttribute returns one of EnableDnsHostnames, EnableDnsSupport, or
// EnableNetworkAddressUsageMetrics. The attribute name in the request uses
// the AWS lowercase-camel form (e.g. "enableDnsHostnames").
func (s *Service) DescribeVpcAttribute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	vpcID := r.Form.Get("VpcId")
	attribute := r.Form.Get("Attribute")

	if vpcID == "" || attribute == "" {
		writeError(w, errInvalidParameter, "VpcId and Attribute are required", http.StatusBadRequest)

		return
	}

	vpcs, err := s.storage.DescribeVpcs(r.Context(), []string{vpcID})
	if err != nil {
		handleError(w, err)

		return
	}

	if len(vpcs) == 0 {
		writeError(w, "InvalidVpcID.NotFound", "The vpc ID '"+vpcID+"' does not exist", http.StatusBadRequest)

		return
	}

	vpc := vpcs[0]

	resp := struct {
		XMLName                          xml.Name      `xml:"DescribeVpcAttributeResponse"`
		Xmlns                            string        `xml:"xmlns,attr"`
		RequestID                        string        `xml:"requestId"`
		VpcID                            string        `xml:"vpcId"`
		EnableDNSHostnames               *xmlBoolValue `xml:"enableDnsHostnames,omitempty"`
		EnableDNSSupport                 *xmlBoolValue `xml:"enableDnsSupport,omitempty"`
		EnableNetworkAddressUsageMetrics *xmlBoolValue `xml:"enableNetworkAddressUsageMetrics,omitempty"`
	}{Xmlns: ec2XMLNS, RequestID: uuid.New().String(), VpcID: vpc.VpcID}

	switch attribute {
	case "enableDnsHostnames":
		resp.EnableDNSHostnames = &xmlBoolValue{Value: vpc.EnableDNSHostnames}
	case "enableDnsSupport":
		resp.EnableDNSSupport = &xmlBoolValue{Value: vpc.EnableDNSSupport}
	case "enableNetworkAddressUsageMetrics":
		resp.EnableNetworkAddressUsageMetrics = &xmlBoolValue{Value: false}
	default:
		writeError(w, errInvalidParameter, "Unsupported attribute: "+attribute, http.StatusBadRequest)

		return
	}

	writeEC2XMLResponse(w, resp)
}

// xmlBoolValue wraps a boolean in the AWS <value>true</value> child shape.
type xmlBoolValue struct {
	Value bool `xml:"value"`
}

// ModifySubnetAttribute handles the ModifySubnetAttribute action.
func (s *Service) ModifySubnetAttribute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	subnetID := r.Form.Get("SubnetId")
	if subnetID == "" {
		writeError(w, errInvalidParameter, "SubnetId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.ModifySubnetAttribute(r.Context(), subnetID, subnetAttributeUpdates(r.Form)); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, struct {
		XMLName   xml.Name `xml:"ModifySubnetAttributeResponse"`
		Xmlns     string   `xml:"xmlns,attr"`
		RequestID string   `xml:"requestId"`
		Return    bool     `xml:"return"`
	}{Xmlns: ec2XMLNS, RequestID: uuid.New().String(), Return: true})
}

// vpcAttributeUpdates extracts EnableDnsHostnames.Value and EnableDnsSupport.Value
// from the form, returning a struct with only the supplied fields populated.
func vpcAttributeUpdates(form map[string][]string) VpcAttributeUpdates {
	var u VpcAttributeUpdates

	if v := getFormBoolPtr(form, "EnableDnsHostnames.Value"); v != nil {
		u.EnableDNSHostnames = v
	}

	if v := getFormBoolPtr(form, "EnableDnsSupport.Value"); v != nil {
		u.EnableDNSSupport = v
	}

	return u
}

// subnetAttributeUpdates extracts MapPublicIpOnLaunch.Value and
// AssignIpv6AddressOnCreation.Value from the form.
func subnetAttributeUpdates(form map[string][]string) SubnetAttributeUpdates {
	var u SubnetAttributeUpdates

	if v := getFormBoolPtr(form, "MapPublicIpOnLaunch.Value"); v != nil {
		u.MapPublicIPOnLaunch = v
	}

	if v := getFormBoolPtr(form, "AssignIpv6AddressOnCreation.Value"); v != nil {
		u.AssignIPv6AddressOnCreation = v
	}

	return u
}

// getFormBoolPtr returns a pointer to a parsed bool from the form, or nil
// if the key is missing or unparseable.
func getFormBoolPtr(form map[string][]string, key string) *bool {
	values, ok := form[key]
	if !ok || len(values) == 0 {
		return nil
	}

	b, err := strconv.ParseBool(values[0])
	if err != nil {
		return nil
	}

	return &b
}

// readTagRequestForm extracts ResourceId.N and Tag.N.Key/Value pairs from the
// already-parsed AWS Query form. The Query dispatcher calls ParseForm before
// dispatch, but ParseForm is idempotent, so calling it again is safe.
func readTagRequestForm(r *http.Request) ([]string, []Tag, error) {
	if err := r.ParseForm(); err != nil {
		return nil, nil, fmt.Errorf("failed to parse form: %w", err)
	}

	resources := parseIndexedListFromForm(r.Form, "ResourceId")
	tags := parseTagsFromForm(r.Form, "Tag")

	return resources, tags, nil
}

// parseIndexedListFromForm returns values for keys of the form "<prefix>.N",
// ordered by N. Indexes start at 1 in AWS Query but we tolerate any positive
// integer. Missing indexes are skipped.
func parseIndexedListFromForm(form map[string][]string, prefix string) []string {
	type entry struct {
		idx int
		val string
	}

	entries := make([]entry, 0)

	for key, values := range form {
		suffix, ok := strings.CutPrefix(key, prefix+".")
		if !ok || len(values) == 0 {
			continue
		}

		// Reject nested keys like "Tag.1.Key" — only direct numeric suffix counts.
		if strings.Contains(suffix, ".") {
			continue
		}

		n, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}

		entries = append(entries, entry{idx: n, val: values[0]})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].idx < entries[j].idx })

	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.val)
	}

	return out
}

// parseTagsFromForm reads "<prefix>.N.Key" and "<prefix>.N.Value" pairs.
// A missing Value is treated as empty string.
func parseTagsFromForm(form map[string][]string, prefix string) []Tag {
	keysByIdx := make(map[int]string)
	valsByIdx := make(map[int]string)

	for key, values := range form {
		suffix, ok := strings.CutPrefix(key, prefix+".")
		if !ok || len(values) == 0 {
			continue
		}

		dot := strings.Index(suffix, ".")
		if dot < 0 {
			continue
		}

		n, err := strconv.Atoi(suffix[:dot])
		if err != nil {
			continue
		}

		switch suffix[dot+1:] {
		case "Key":
			keysByIdx[n] = values[0]
		case "Value":
			valsByIdx[n] = values[0]
		}
	}

	indexes := make([]int, 0, len(keysByIdx))
	for n := range keysByIdx {
		indexes = append(indexes, n)
	}

	sort.Ints(indexes)

	tags := make([]Tag, 0, len(indexes))
	for _, n := range indexes {
		tags = append(tags, Tag{Key: keysByIdx[n], Value: valsByIdx[n]})
	}

	return tags
}

// parseFiltersFromForm reads "Filter.N.Name" and "Filter.N.Value.M" entries
// into a Name -> []Value map.
func parseFiltersFromForm(form map[string][]string) map[string][]string {
	type filterAcc struct {
		name   string
		values []string
	}

	byIdx := make(map[int]*filterAcc)

	for key, values := range form {
		suffix, ok := strings.CutPrefix(key, "Filter.")
		if !ok || len(values) == 0 {
			continue
		}

		dot := strings.Index(suffix, ".")
		if dot < 0 {
			continue
		}

		n, err := strconv.Atoi(suffix[:dot])
		if err != nil {
			continue
		}

		field := suffix[dot+1:]

		entry, ok := byIdx[n]
		if !ok {
			entry = &filterAcc{}
			byIdx[n] = entry
		}

		switch {
		case field == "Name":
			entry.name = values[0]
		case strings.HasPrefix(field, "Value."):
			entry.values = append(entry.values, values[0])
		}
	}

	out := make(map[string][]string)

	for _, entry := range byIdx {
		if entry.name == "" {
			continue
		}

		out[entry.name] = append(out[entry.name], entry.values...)
	}

	return out
}

// tagSpec accumulates one TagSpecification.N entry being parsed from form data.
type tagSpec struct {
	resourceType string
	tags         []Tag
}

// parseTagSpecificationsForResourceType reads TagSpecifications.N.ResourceType
// and TagSpecifications.N.Tag.M.{Key,Value} from form data, returning the
// tags whose ResourceType matches `resourceType` (e.g. "vpc", "subnet").
// The Query form-to-JSON converter does not understand this nested pattern,
// so callers parse from r.Form directly.
func parseTagSpecificationsForResourceType(form map[string][]string, resourceType string) []Tag {
	specs := make(map[int]*tagSpec)

	for key, values := range form {
		applyTagSpecFormEntry(specs, key, values)
	}

	for _, sp := range specs {
		if sp.resourceType == resourceType {
			return sp.tags
		}
	}

	return nil
}

func applyTagSpecFormEntry(specs map[int]*tagSpec, key string, values []string) {
	suffix, ok := strings.CutPrefix(key, "TagSpecification.")
	if !ok || len(values) == 0 {
		return
	}

	dot := strings.Index(suffix, ".")
	if dot < 0 {
		return
	}

	n, err := strconv.Atoi(suffix[:dot])
	if err != nil {
		return
	}

	entry, exists := specs[n]
	if !exists {
		entry = &tagSpec{}
		specs[n] = entry
	}

	setTagSpecField(entry, suffix[dot+1:], values[0])
}

func setTagSpecField(entry *tagSpec, field, value string) {
	switch {
	case field == "ResourceType":
		entry.resourceType = value
	case strings.HasPrefix(field, "Tag."):
		applyTagFromTagSpec(entry, strings.TrimPrefix(field, "Tag."), value)
	}
}

func applyTagFromTagSpec(entry *tagSpec, suffix, value string) {
	tdot := strings.Index(suffix, ".")
	if tdot < 0 {
		return
	}

	m, err := strconv.Atoi(suffix[:tdot])
	if err != nil {
		return
	}

	ensureTag(&entry.tags, m)

	switch suffix[tdot+1:] {
	case "Key":
		entry.tags[m-1].Key = value
	case "Value":
		entry.tags[m-1].Value = value
	}
}

func ensureTag(tags *[]Tag, n int) {
	for len(*tags) < n {
		*tags = append(*tags, Tag{})
	}
}

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)
	handler := s.getActionHandler(action)

	if handler == nil {
		writeError(w, errInvalidAction, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// getActionHandler returns the handler function for the given action.
func (s *Service) getActionHandler(action string) func(http.ResponseWriter, *http.Request) {
	handlers := map[string]func(http.ResponseWriter, *http.Request){
		// Instance operations
		"RunInstances":       s.RunInstances,
		"TerminateInstances": s.TerminateInstances,
		"DescribeInstances":  s.DescribeInstances,
		"StartInstances":     s.StartInstances,
		"StopInstances":      s.StopInstances,
		// Security group operations
		"CreateSecurityGroup":           s.CreateSecurityGroup,
		"DeleteSecurityGroup":           s.DeleteSecurityGroup,
		"AuthorizeSecurityGroupIngress": s.AuthorizeSecurityGroupIngress,
		"AuthorizeSecurityGroupEgress":  s.AuthorizeSecurityGroupEgress,
		"DescribeSecurityGroups":        s.DescribeSecurityGroups,
		"RevokeSecurityGroupIngress":    s.RevokeSecurityGroupIngress,
		"RevokeSecurityGroupEgress":     s.RevokeSecurityGroupEgress,
		// Key pair operations
		"CreateKeyPair":    s.CreateKeyPair,
		"DeleteKeyPair":    s.DeleteKeyPair,
		"DescribeKeyPairs": s.DescribeKeyPairs,
		// VPC operations
		"CreateVpc":    s.CreateVpc,
		"DeleteVpc":    s.DeleteVpc,
		"DescribeVpcs": s.DescribeVpcs,
		// Subnet operations
		"CreateSubnet":    s.CreateSubnet,
		"DeleteSubnet":    s.DeleteSubnet,
		"DescribeSubnets": s.DescribeSubnets,
		// Internet gateway operations
		"CreateInternetGateway":    s.CreateInternetGateway,
		"AttachInternetGateway":    s.AttachInternetGateway,
		"DetachInternetGateway":    s.DetachInternetGateway,
		"DeleteInternetGateway":    s.DeleteInternetGateway,
		"DescribeInternetGateways": s.DescribeInternetGateways,
		// Network interface — destroy-time stub
		"DescribeNetworkInterfaces": s.DescribeNetworkInterfaces,
		// Route table operations
		"CreateRouteTable":    s.CreateRouteTable,
		"CreateRoute":         s.CreateRoute,
		"AssociateRouteTable": s.AssociateRouteTable,
		"DescribeRouteTables": s.DescribeRouteTables,
		// NAT gateway operations
		"CreateNatGateway":    s.CreateNatGateway,
		"DescribeNatGateways": s.DescribeNatGateways,
		// Tag operations
		"CreateTags":   s.CreateTags,
		"DeleteTags":   s.DeleteTags,
		"DescribeTags": s.DescribeTags,
		// VPC / Subnet attribute mutation
		"ModifyVpcAttribute":    s.ModifyVpcAttribute,
		"DescribeVpcAttribute":  s.DescribeVpcAttribute,
		"ModifySubnetAttribute": s.ModifySubnetAttribute,
	}

	return handlers[action]
}

// convertToXMLInstance converts an Instance to XMLInstance.
func convertToXMLInstance(inst *Instance) XMLInstance {
	groupSet := make([]XMLGroupIdentifier, 0, len(inst.SecurityGroups))
	for _, sg := range inst.SecurityGroups {
		groupSet = append(groupSet, XMLGroupIdentifier(sg))
	}

	return XMLInstance{
		InstanceID:       inst.InstanceID,
		ImageID:          inst.ImageID,
		InstanceType:     inst.InstanceType,
		InstanceState:    XMLInstanceState{Code: inst.State.Code, Name: inst.State.Name},
		PrivateIPAddress: inst.PrivateIPAddress,
		IPAddress:        inst.PublicIPAddress,
		KeyName:          inst.KeyName,
		LaunchTime:       inst.LaunchTime.Format("2006-01-02T15:04:05.000Z"),
		GroupSet:         XMLGroupSet{Items: groupSet},
	}
}

// convertToXMLInstanceStateChangeSet converts instance state changes to XML format.
func convertToXMLInstanceStateChangeSet(changes []InstanceStateChange) XMLInstanceStateChangeSet {
	items := make([]XMLInstanceStateChange, 0, len(changes))
	for _, c := range changes {
		items = append(items, XMLInstanceStateChange{
			InstanceID:    c.InstanceID,
			CurrentState:  XMLInstanceState{Code: c.CurrentState.Code, Name: c.CurrentState.Name},
			PreviousState: XMLInstanceState{Code: c.PreviousState.Code, Name: c.PreviousState.Name},
		})
	}

	return XMLInstanceStateChangeSet{Items: items}
}

// readEC2JSONRequest reads and decodes JSON request body.
func readEC2JSONRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// extractAction extracts the action name from the request.
// It tries X-Amz-Target header first (set by QueryProtocolDispatcher),
// then falls back to URL query parameter.
func extractAction(r *http.Request) string {
	// Try X-Amz-Target header (format: "AmazonEC2.ActionName").
	target := r.Header.Get("X-Amz-Target")
	if target != "" {
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			return target[idx+1:]
		}
	}

	// Fallback to URL query parameter.
	return r.URL.Query().Get("Action")
}

// writeEC2XMLResponse writes an XML response with HTTP 200 OK.
func writeEC2XMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// writeError writes an EC2 error response in XML format.
func writeError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Errors: XMLErrors{
			Error: XMLError{
				Code:    code,
				Message: message,
			},
		},
		RequestID: requestID,
	})
}

// handleError handles EC2 errors and writes the appropriate response.
func handleError(w http.ResponseWriter, err error) {
	var ec2Err *Error
	if errors.As(err, &ec2Err) {
		writeError(w, ec2Err.Code, ec2Err.Message, http.StatusBadRequest)

		return
	}

	writeError(w, errInternalError, "Internal server error", http.StatusInternalServerError)
}

// convertToXMLVpc converts a Vpc to XMLVpc.
func convertToXMLVpc(vpc *Vpc) XMLVpc {
	tags := make([]XMLTag, 0, len(vpc.Tags))
	for _, t := range vpc.Tags {
		tags = append(tags, XMLTag(t))
	}

	return XMLVpc{
		VpcID:           vpc.VpcID,
		CidrBlock:       vpc.CidrBlock,
		State:           vpc.State,
		IsDefault:       vpc.IsDefault,
		InstanceTenancy: vpc.InstanceTenancy,
		TagSet:          XMLTagSet{Items: tags},
	}
}

// convertToXMLSubnet converts a Subnet to XMLSubnet.
func convertToXMLSubnet(subnet *Subnet) XMLSubnet {
	tags := make([]XMLTag, 0, len(subnet.Tags))
	for _, t := range subnet.Tags {
		tags = append(tags, XMLTag(t))
	}

	return XMLSubnet{
		SubnetID:                subnet.SubnetID,
		VpcID:                   subnet.VpcID,
		CidrBlock:               subnet.CidrBlock,
		AvailabilityZone:        subnet.AvailabilityZone,
		AvailableIPAddressCount: subnet.AvailableIPAddressCount,
		State:                   subnet.State,
		MapPublicIPOnLaunch:     subnet.MapPublicIPOnLaunch,
		TagSet:                  XMLTagSet{Items: tags},
	}
}

// convertToXMLInternetGateway converts an InternetGateway to XMLInternetGateway.
func convertToXMLInternetGateway(igw *InternetGateway) XMLInternetGateway {
	tags := make([]XMLTag, 0, len(igw.Tags))
	for _, t := range igw.Tags {
		tags = append(tags, XMLTag(t))
	}

	attachments := make([]XMLInternetGatewayAttachment, 0, len(igw.Attachments))
	for _, a := range igw.Attachments {
		attachments = append(attachments, XMLInternetGatewayAttachment(a))
	}

	return XMLInternetGateway{
		InternetGatewayID: igw.InternetGatewayID,
		AttachmentSet:     XMLInternetGatewayAttachmentSet{Items: attachments},
		TagSet:            XMLTagSet{Items: tags},
	}
}

// convertToXMLRouteTable converts a RouteTable to XMLRouteTable.
func convertToXMLRouteTable(rt *RouteTable) XMLRouteTable {
	tags := make([]XMLTag, 0, len(rt.Tags))
	for _, t := range rt.Tags {
		tags = append(tags, XMLTag(t))
	}

	routes := make([]XMLRoute, 0, len(rt.Routes))
	for _, r := range rt.Routes {
		routes = append(routes, XMLRoute(r))
	}

	associations := make([]XMLRouteTableAssociation, 0, len(rt.Associations))
	for _, a := range rt.Associations {
		associations = append(associations, XMLRouteTableAssociation(a))
	}

	return XMLRouteTable{
		RouteTableID:   rt.RouteTableID,
		VpcID:          rt.VpcID,
		RouteSet:       XMLRouteSet{Items: routes},
		AssociationSet: XMLRouteTableAssociationSet{Items: associations},
		TagSet:         XMLTagSet{Items: tags},
	}
}

// RevokeSecurityGroupIngress removes the requested rules / CIDRs
// from the SG's ingress list. terraform aws_security_group calls this
// on every Update where cidr_blocks shrinks, so the side effect must
// reach storage — earlier this was a no-op stub which left stale
// rules behind for the audit-style consumers to trip over.
//
// The form-converter dotted-key issue applies here too: re-parse the
// form so IpPermissions.N.* lands in the request.
func (s *Service) RevokeSecurityGroupIngress(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeSecurityGroupIngressRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.GroupID == "" && req.GroupName == "" {
		writeError(w, errInvalidParameter, "GroupId or GroupName is required", http.StatusBadRequest)

		return
	}

	if err := r.ParseForm(); err == nil {
		if perms := parseIPPermissionsFromForm(r.Form); len(perms) > 0 {
			req.IPPermissions = perms
		}
	}

	if err := s.storage.RevokeSecurityGroupIngress(r.Context(), req.GroupID, req.GroupName, req.IPPermissions); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, struct {
		XMLName   xml.Name `xml:"RevokeSecurityGroupIngressResponse"`
		Xmlns     string   `xml:"xmlns,attr"`
		RequestID string   `xml:"requestId"`
		Return    bool     `xml:"return"`
	}{Xmlns: ec2XMLNS, RequestID: uuid.New().String(), Return: true})
}

// RevokeSecurityGroupEgress is the egress mirror. Egress lookup is by
// GroupID only — AWS doesn't accept GroupName for egress, mirroring
// the existing Authorize handler.
func (s *Service) RevokeSecurityGroupEgress(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeSecurityGroupEgressRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.GroupID == "" {
		writeError(w, errInvalidParameter, "GroupId is required", http.StatusBadRequest)

		return
	}

	if err := r.ParseForm(); err == nil {
		if perms := parseIPPermissionsFromForm(r.Form); len(perms) > 0 {
			req.IPPermissions = perms
		}
	}

	if err := s.storage.RevokeSecurityGroupEgress(r.Context(), req.GroupID, req.IPPermissions); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, struct {
		XMLName   xml.Name `xml:"RevokeSecurityGroupEgressResponse"`
		Xmlns     string   `xml:"xmlns,attr"`
		RequestID string   `xml:"requestId"`
		Return    bool     `xml:"return"`
	}{Xmlns: ec2XMLNS, RequestID: uuid.New().String(), Return: true})
}

// DescribeSecurityGroups handles the DescribeSecurityGroups action.
// Filters honoured: GroupId.N, GroupName.N. Without either, every SG
// in storage is returned.
func (s *Service) DescribeSecurityGroups(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	groupIDs := parseIndexedListFromForm(r.Form, "GroupId")
	groupNames := parseIndexedListFromForm(r.Form, "GroupName")

	sgs, err := s.storage.DescribeSecurityGroups(r.Context(), groupIDs, groupNames)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]XMLSecurityGroup, 0, len(sgs))
	for _, sg := range sgs {
		items = append(items, convertToXMLSecurityGroup(sg))
	}

	writeEC2XMLResponse(w, XMLDescribeSecurityGroupsResponse{
		Xmlns:             ec2XMLNS,
		RequestID:         uuid.New().String(),
		SecurityGroupInfo: XMLSecurityGroupInfo{Items: items},
	})
}

// convertToXMLSecurityGroup is the SecurityGroup → wire-shape adapter.
// The IngressRules / EgressRules are copied through ipPermission()
// which handles the IpRanges nested list.
func convertToXMLSecurityGroup(sg *SecurityGroup) XMLSecurityGroup {
	tags := make([]XMLTag, 0, len(sg.Tags))
	for _, t := range sg.Tags {
		tags = append(tags, XMLTag(t))
	}

	return XMLSecurityGroup{
		OwnerID:             defaultAccountID,
		GroupID:             sg.GroupID,
		GroupName:           sg.GroupName,
		GroupDescription:    sg.Description,
		VpcID:               sg.VpcID,
		IPPermissions:       XMLIPPermissionSet{Items: ipPermissionsToXML(sg.IngressRules)},
		IPPermissionsEgress: XMLIPPermissionSet{Items: ipPermissionsToXML(sg.EgressRules)},
		TagSet:              XMLTagSet{Items: tags},
	}
}

// ipPermissionsToXML translates each IPPermission into its wire form.
func ipPermissionsToXML(perms []IPPermission) []XMLIPPermission {
	out := make([]XMLIPPermission, 0, len(perms))

	for _, p := range perms {
		ranges := make([]XMLIPRange, 0, len(p.IPRanges))
		for _, r := range p.IPRanges {
			ranges = append(ranges, XMLIPRange(r))
		}

		out = append(out, XMLIPPermission{
			IPProtocol: p.IPProtocol,
			FromPort:   p.FromPort,
			ToPort:     p.ToPort,
			IPRanges:   XMLIPRanges{Items: ranges},
		})
	}

	return out
}

// parseIPPermissionsFromForm reads the AWS Query wire shape for an
// IpPermissions list — IpPermissions.N.{IpProtocol,FromPort,ToPort}
// plus IpPermissions.N.IpRanges.M.CidrIp — into the slice the storage
// layer expects. The shared form-to-JSON converter only handles flat
// indexed scalar lists, not the nested member.N.<field> shape, so the
// (Authorize|Revoke)SecurityGroup{Ingress,Egress} handlers need this
// helper to recover the rules from r.Form directly.
func parseIPPermissionsFromForm(form map[string][]string) []IPPermission {
	byIdx := make(map[int]*IPPermission)

	for key, values := range form {
		applyIPPermissionFormEntry(byIdx, key, values)
	}

	indexes := make([]int, 0, len(byIdx))
	for n := range byIdx {
		indexes = append(indexes, n)
	}

	sort.Ints(indexes)

	out := make([]IPPermission, 0, len(indexes))
	for _, n := range indexes {
		out = append(out, *byIdx[n])
	}

	return out
}

// applyIPPermissionFormEntry is a single-key apply for the
// IpPermissions.N.X form shape. Only keys under that prefix are
// considered; everything else is ignored so unrelated form fields
// (e.g. GroupId) don't pollute the result.
func applyIPPermissionFormEntry(byIdx map[int]*IPPermission, key string, values []string) {
	suffix, ok := strings.CutPrefix(key, "IpPermissions.")
	if !ok || len(values) == 0 {
		return
	}

	dot := strings.Index(suffix, ".")
	if dot < 0 {
		return
	}

	n, err := strconv.Atoi(suffix[:dot])
	if err != nil || n < 1 {
		return
	}

	entry, exists := byIdx[n]
	if !exists {
		entry = &IPPermission{}
		byIdx[n] = entry
	}

	setIPPermissionField(entry, suffix[dot+1:], values[0])
}

// setIPPermissionField applies one .X field on a single permission.
// IpRanges is special-cased: each IpRanges.M.CidrIp encountered grows
// the slice, so out-of-order M values still produce one entry each.
func setIPPermissionField(entry *IPPermission, field, value string) {
	switch {
	case field == "IpProtocol":
		entry.IPProtocol = value
	case field == "FromPort":
		if v, err := strconv.Atoi(value); err == nil {
			entry.FromPort = v
		}
	case field == "ToPort":
		if v, err := strconv.Atoi(value); err == nil {
			entry.ToPort = v
		}
	case strings.HasPrefix(field, "IpRanges."):
		rest := strings.TrimPrefix(field, "IpRanges.")

		rdot := strings.Index(rest, ".")
		if rdot < 0 {
			return
		}

		if rest[rdot+1:] == "CidrIp" {
			entry.IPRanges = append(entry.IPRanges, IPRange{CidrIP: value})
		}
	}
}

// convertToXMLNatGateway converts a NatGateway to XMLNatGateway.
func convertToXMLNatGateway(natgw *NatGateway) XMLNatGateway {
	tags := make([]XMLTag, 0, len(natgw.Tags))
	for _, t := range natgw.Tags {
		tags = append(tags, XMLTag(t))
	}

	return XMLNatGateway{
		NatGatewayID:     natgw.NatGatewayID,
		SubnetID:         natgw.SubnetID,
		VpcID:            natgw.VpcID,
		State:            natgw.State,
		ConnectivityType: natgw.ConnectivityType,
		TagSet:           XMLTagSet{Items: tags},
	}
}
