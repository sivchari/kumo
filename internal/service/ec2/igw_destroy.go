package ec2

import (
	"net/http"

	"github.com/google/uuid"
)

// DetachInternetGateway handles the DetachInternetGateway action.
//
// terraform-provider-aws calls this during destroy of aws_internet_gateway,
// after which it issues DeleteInternetGateway. Without this handler the
// destroy fails with InvalidAction and the IGW leaks in kumo.
func (s *Service) DetachInternetGateway(w http.ResponseWriter, r *http.Request) {
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

	if err := s.storage.DetachInternetGateway(r.Context(), req.InternetGatewayID, req.VpcID); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLDetachInternetGatewayResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// DeleteInternetGateway handles the DeleteInternetGateway action.
func (s *Service) DeleteInternetGateway(w http.ResponseWriter, r *http.Request) {
	var req DeleteInternetGatewayRequest
	if err := readEC2JSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.InternetGatewayID == "" {
		writeError(w, errInvalidParameter, "InternetGatewayId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteInternetGateway(r.Context(), req.InternetGatewayID); err != nil {
		handleError(w, err)

		return
	}

	writeEC2XMLResponse(w, XMLDeleteInternetGatewayResponse{
		Xmlns:     ec2XMLNS,
		RequestID: uuid.New().String(),
		Return:    true,
	})
}

// DescribeNetworkInterfaces returns an empty NetworkInterfaceSet.
//
// terraform-provider-aws calls this during destroy of any subnet / IGW to
// check for dangling ENIs that would block the delete (lingering
// load-balancer ENIs, lambda VPC ENIs, etc.). kumo does not model ENIs
// yet; reporting "no ENIs" lets destroy proceed.
func (s *Service) DescribeNetworkInterfaces(w http.ResponseWriter, _ *http.Request) {
	writeEC2XMLResponse(w, XMLDescribeNetworkInterfacesResponse{
		Xmlns:               ec2XMLNS,
		RequestID:           uuid.New().String(),
		NetworkInterfaceSet: XMLNetworkInterfaceSet{Items: []XMLNetworkInterface{}},
	})
}
