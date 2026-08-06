// Package appmesh provides the AWS App Mesh service implementation.
package appmesh

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	meshPathPrefix = "/v20190125/meshes/"
)

// --- Mesh Handlers ---

// CreateMesh handles the CreateMesh API operation.
func (s *Service) CreateMesh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateMeshInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	mesh, err := s.storage.CreateMesh(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, mesh)
}

// DescribeMesh handles the DescribeMesh API operation.
func (s *Service) DescribeMesh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	mesh, err := s.storage.DescribeMesh(ctx, meshName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, mesh)
}

// ListMeshes handles the ListMeshes API operation.
func (s *Service) ListMeshes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	req := &ListMeshesInput{
		NextToken: query.Get("nextToken"),
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			writeError(w, errBadRequestException, "Invalid limit parameter", http.StatusBadRequest)

			return
		}

		req.Limit = int32(limit)
	}

	output, err := s.storage.ListMeshes(ctx, req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, output)
}

// UpdateMesh handles the UpdateMesh API operation.
func (s *Service) UpdateMesh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	var req UpdateMeshInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName

	mesh, err := s.storage.UpdateMesh(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, mesh)
}

// DeleteMesh handles the DeleteMesh API operation.
func (s *Service) DeleteMesh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	mesh, err := s.storage.DeleteMesh(ctx, meshName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, mesh)
}

// --- Virtual Node Handlers ---

// CreateVirtualNode handles the CreateVirtualNode API operation.
func (s *Service) CreateVirtualNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	var req CreateVirtualNodeInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName

	node, err := s.storage.CreateVirtualNode(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, node)
}

// DescribeVirtualNode handles the DescribeVirtualNode API operation.
func (s *Service) DescribeVirtualNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualNodeName := extractMeshAndResourceName(r.URL.Path, "virtualNodes")
	if meshName == "" || virtualNodeName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual node name are required", http.StatusBadRequest)

		return
	}

	node, err := s.storage.DescribeVirtualNode(ctx, meshName, virtualNodeName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, node)
}

// ListVirtualNodes handles the ListVirtualNodes API operation.
func (s *Service) ListVirtualNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	query := r.URL.Query()
	req := &ListVirtualNodesInput{
		MeshName:  meshName,
		NextToken: query.Get("nextToken"),
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			writeError(w, errBadRequestException, "Invalid limit parameter", http.StatusBadRequest)

			return
		}

		req.Limit = int32(limit)
	}

	output, err := s.storage.ListVirtualNodes(ctx, req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, output)
}

// UpdateVirtualNode handles the UpdateVirtualNode API operation.
func (s *Service) UpdateVirtualNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualNodeName := extractMeshAndResourceName(r.URL.Path, "virtualNodes")
	if meshName == "" || virtualNodeName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual node name are required", http.StatusBadRequest)

		return
	}

	var req UpdateVirtualNodeInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName
	req.VirtualNodeName = virtualNodeName

	node, err := s.storage.UpdateVirtualNode(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, node)
}

// DeleteVirtualNode handles the DeleteVirtualNode API operation.
func (s *Service) DeleteVirtualNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualNodeName := extractMeshAndResourceName(r.URL.Path, "virtualNodes")
	if meshName == "" || virtualNodeName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual node name are required", http.StatusBadRequest)

		return
	}

	node, err := s.storage.DeleteVirtualNode(ctx, meshName, virtualNodeName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, node)
}

// --- Virtual Service Handlers ---

// CreateVirtualService handles the CreateVirtualService API operation.
func (s *Service) CreateVirtualService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	var req CreateVirtualServiceInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName

	service, err := s.storage.CreateVirtualService(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, service)
}

// DescribeVirtualService handles the DescribeVirtualService API operation.
func (s *Service) DescribeVirtualService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualServiceName := extractMeshAndResourceName(r.URL.Path, "virtualServices")
	if meshName == "" || virtualServiceName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual service name are required", http.StatusBadRequest)

		return
	}

	service, err := s.storage.DescribeVirtualService(ctx, meshName, virtualServiceName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, service)
}

// ListVirtualServices handles the ListVirtualServices API operation.
func (s *Service) ListVirtualServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	query := r.URL.Query()
	req := &ListVirtualServicesInput{
		MeshName:  meshName,
		NextToken: query.Get("nextToken"),
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			writeError(w, errBadRequestException, "Invalid limit parameter", http.StatusBadRequest)

			return
		}

		req.Limit = int32(limit)
	}

	output, err := s.storage.ListVirtualServices(ctx, req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, output)
}

// UpdateVirtualService handles the UpdateVirtualService API operation.
func (s *Service) UpdateVirtualService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualServiceName := extractMeshAndResourceName(r.URL.Path, "virtualServices")
	if meshName == "" || virtualServiceName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual service name are required", http.StatusBadRequest)

		return
	}

	var req UpdateVirtualServiceInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName
	req.VirtualServiceName = virtualServiceName

	service, err := s.storage.UpdateVirtualService(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, service)
}

// DeleteVirtualService handles the DeleteVirtualService API operation.
func (s *Service) DeleteVirtualService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualServiceName := extractMeshAndResourceName(r.URL.Path, "virtualServices")
	if meshName == "" || virtualServiceName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual service name are required", http.StatusBadRequest)

		return
	}

	service, err := s.storage.DeleteVirtualService(ctx, meshName, virtualServiceName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, service)
}

// --- Virtual Router Handlers ---

// CreateVirtualRouter handles the CreateVirtualRouter API operation.
func (s *Service) CreateVirtualRouter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	var req CreateVirtualRouterInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName

	router, err := s.storage.CreateVirtualRouter(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, router)
}

// DescribeVirtualRouter handles the DescribeVirtualRouter API operation.
func (s *Service) DescribeVirtualRouter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualRouterName := extractMeshAndResourceName(r.URL.Path, "virtualRouters")
	if meshName == "" || virtualRouterName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual router name are required", http.StatusBadRequest)

		return
	}

	router, err := s.storage.DescribeVirtualRouter(ctx, meshName, virtualRouterName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, router)
}

// ListVirtualRouters handles the ListVirtualRouters API operation.
func (s *Service) ListVirtualRouters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName := extractMeshName(r.URL.Path)
	if meshName == "" {
		writeError(w, errBadRequestException, "Mesh name is required", http.StatusBadRequest)

		return
	}

	query := r.URL.Query()
	req := &ListVirtualRoutersInput{
		MeshName:  meshName,
		NextToken: query.Get("nextToken"),
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			writeError(w, errBadRequestException, "Invalid limit parameter", http.StatusBadRequest)

			return
		}

		req.Limit = int32(limit)
	}

	output, err := s.storage.ListVirtualRouters(ctx, req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, output)
}

// UpdateVirtualRouter handles the UpdateVirtualRouter API operation.
func (s *Service) UpdateVirtualRouter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualRouterName := extractMeshAndResourceName(r.URL.Path, "virtualRouters")
	if meshName == "" || virtualRouterName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual router name are required", http.StatusBadRequest)

		return
	}

	var req UpdateVirtualRouterInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName
	req.VirtualRouterName = virtualRouterName

	router, err := s.storage.UpdateVirtualRouter(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, router)
}

// DeleteVirtualRouter handles the DeleteVirtualRouter API operation.
func (s *Service) DeleteVirtualRouter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualRouterName := extractMeshAndResourceName(r.URL.Path, "virtualRouters")
	if meshName == "" || virtualRouterName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual router name are required", http.StatusBadRequest)

		return
	}

	router, err := s.storage.DeleteVirtualRouter(ctx, meshName, virtualRouterName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, router)
}

// --- Route Handlers ---

// CreateRoute handles the CreateRoute API operation.
func (s *Service) CreateRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualRouterName := extractMeshAndVirtualRouterForRoute(r.URL.Path)
	if meshName == "" || virtualRouterName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual router name are required", http.StatusBadRequest)

		return
	}

	var req CreateRouteInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName
	req.VirtualRouterName = virtualRouterName

	route, err := s.storage.CreateRoute(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, route)
}

// DescribeRoute handles the DescribeRoute API operation.
func (s *Service) DescribeRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualRouterName, routeName := extractRouteComponents(r.URL.Path)
	if meshName == "" || virtualRouterName == "" || routeName == "" {
		writeError(w, errBadRequestException, "Mesh name, virtual router name, and route name are required", http.StatusBadRequest)

		return
	}

	route, err := s.storage.DescribeRoute(ctx, meshName, virtualRouterName, routeName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, route)
}

// ListRoutes handles the ListRoutes API operation.
func (s *Service) ListRoutes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualRouterName := extractMeshAndVirtualRouterForRoute(r.URL.Path)
	if meshName == "" || virtualRouterName == "" {
		writeError(w, errBadRequestException, "Mesh name and virtual router name are required", http.StatusBadRequest)

		return
	}

	query := r.URL.Query()
	req := &ListRoutesInput{
		MeshName:          meshName,
		VirtualRouterName: virtualRouterName,
		NextToken:         query.Get("nextToken"),
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			writeError(w, errBadRequestException, "Invalid limit parameter", http.StatusBadRequest)

			return
		}

		req.Limit = int32(limit)
	}

	output, err := s.storage.ListRoutes(ctx, req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, output)
}

// UpdateRoute handles the UpdateRoute API operation.
func (s *Service) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualRouterName, routeName := extractRouteComponents(r.URL.Path)
	if meshName == "" || virtualRouterName == "" || routeName == "" {
		writeError(w, errBadRequestException, "Mesh name, virtual router name, and route name are required", http.StatusBadRequest)

		return
	}

	var req UpdateRouteInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errBadRequestException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.MeshName = meshName
	req.VirtualRouterName = virtualRouterName
	req.RouteName = routeName

	route, err := s.storage.UpdateRoute(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, route)
}

// DeleteRoute handles the DeleteRoute API operation.
func (s *Service) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meshName, virtualRouterName, routeName := extractRouteComponents(r.URL.Path)
	if meshName == "" || virtualRouterName == "" || routeName == "" {
		writeError(w, errBadRequestException, "Mesh name, virtual router name, and route name are required", http.StatusBadRequest)

		return
	}

	route, err := s.storage.DeleteRoute(ctx, meshName, virtualRouterName, routeName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, route)
}

// --- Helper Functions ---

// extractMeshName extracts the mesh name from a URL path like /v20190125/meshes/{meshName}.
func extractMeshName(path string) string {
	if !strings.HasPrefix(path, meshPathPrefix) {
		return ""
	}

	remainder := strings.TrimPrefix(path, meshPathPrefix)

	if idx := strings.Index(remainder, "/"); idx != -1 {
		remainder = remainder[:idx]
	}

	return remainder
}

// extractMeshAndResourceName extracts mesh name and resource name from paths like
// /v20190125/meshes/{meshName}/{resourceType}/{resourceName}.
func extractMeshAndResourceName(path, resourceType string) (string, string) {
	if !strings.HasPrefix(path, meshPathPrefix) {
		return "", ""
	}

	remainder := strings.TrimPrefix(path, meshPathPrefix)
	parts := strings.Split(remainder, "/")

	// Expect: meshName, resourceType, resourceName
	if len(parts) < 3 || parts[1] != resourceType {
		return "", ""
	}

	return parts[0], parts[2]
}

// extractMeshAndVirtualRouterForRoute extracts mesh name and virtual router name for route operations.
// Path: /v20190125/meshes/{meshName}/virtualRouter/{virtualRouterName}/routes.
func extractMeshAndVirtualRouterForRoute(path string) (string, string) {
	if !strings.HasPrefix(path, meshPathPrefix) {
		return "", ""
	}

	remainder := strings.TrimPrefix(path, meshPathPrefix)
	parts := strings.Split(remainder, "/")

	// Expect: meshName, virtualRouter, routerName, routes...
	if len(parts) < 4 || parts[1] != "virtualRouter" {
		return "", ""
	}

	return parts[0], parts[2]
}

// extractRouteComponents extracts mesh name, virtual router name, and route name.
// Path: /v20190125/meshes/{meshName}/virtualRouter/{virtualRouterName}/routes/{routeName}.
func extractRouteComponents(path string) (string, string, string) {
	if !strings.HasPrefix(path, meshPathPrefix) {
		return "", "", ""
	}

	remainder := strings.TrimPrefix(path, meshPathPrefix)
	parts := strings.Split(remainder, "/")

	// Expect: meshName, virtualRouter, routerName, routes, routeName
	if len(parts) < 5 || parts[1] != "virtualRouter" || parts[3] != "routes" {
		return "", "", ""
	}

	return parts[0], parts[2], parts[4]
}

// writeJSON writes a JSON response with 200 OK status.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, _, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := &ErrorResponse{
		Message: message,
	}

	json.NewEncoder(w).Encode(resp) //nolint:errcheck,gosec // best effort error handling
}

// handleError handles storage errors and writes an appropriate HTTP response.
func handleError(w http.ResponseWriter, err error) {
	var meshErr *Error
	if errors.As(err, &meshErr) {
		writeError(w, meshErr.Code, meshErr.Message, meshErr.HTTPStatusCode())

		return
	}

	writeError(w, errInternalServerException, "Internal server error", http.StatusInternalServerError)
}
