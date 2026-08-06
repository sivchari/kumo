package cloudcontrol

import (
	"net/http"

	"github.com/google/uuid"
)

// CreateResource provisions a resource of the given type from a
// DesiredState JSON document. kumo runs the storage call synchronously,
// so the returned ProgressEvent always reports SUCCESS with the read-back
// ResourceModel attached.
func (s *Service) CreateResource(w http.ResponseWriter, r *http.Request) {
	var input CreateResourceInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, "InvalidRequest", "failed to decode request body: "+err.Error())

		return
	}

	handler, ok := s.registry.Get(input.TypeName)
	if !ok {
		writeError(w, "TypeNotFoundException", "Type "+input.TypeName+" is not supported")

		return
	}

	identifier, state, err := handler.Create(r.Context(), []byte(input.DesiredState))
	if err != nil {
		writeError(w, "GeneralServiceException", err.Error())

		return
	}

	ev := ProgressEvent{
		TypeName:        input.TypeName,
		Identifier:      identifier,
		RequestToken:    requestToken(input.ClientToken),
		Operation:       "CREATE",
		OperationStatus: "SUCCESS",
		EventTime:       nowEpoch(),
		ResourceModel:   string(state),
	}
	s.progress.record(&ev)
	writeJSON(w, ProgressEventOutput{ProgressEvent: ev})
}

// GetResource returns the current state of the resource. Cloud Control
// uses Get for synchronous reads; status polling is GetResourceRequestStatus.
func (s *Service) GetResource(w http.ResponseWriter, r *http.Request) {
	var input GetResourceInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, "InvalidRequest", "failed to decode request body: "+err.Error())

		return
	}

	handler, ok := s.registry.Get(input.TypeName)
	if !ok {
		writeError(w, "TypeNotFoundException", "Type "+input.TypeName+" is not supported")

		return
	}

	state, err := handler.Read(r.Context(), input.Identifier)
	if err != nil {
		if IsNotFound(err) {
			writeError(w, "ResourceNotFoundException", err.Error())

			return
		}

		writeError(w, "GeneralServiceException", err.Error())

		return
	}

	writeJSON(w, GetResourceOutput{
		TypeName: input.TypeName,
		ResourceDescription: ResourceDescriptionWire{
			Identifier: input.Identifier,
			Properties: string(state),
		},
	})
}

// UpdateResource applies an RFC 6902 patch to the existing resource.
func (s *Service) UpdateResource(w http.ResponseWriter, r *http.Request) {
	var input UpdateResourceInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, "InvalidRequest", "failed to decode request body: "+err.Error())

		return
	}

	handler, ok := s.registry.Get(input.TypeName)
	if !ok {
		writeError(w, "TypeNotFoundException", "Type "+input.TypeName+" is not supported")

		return
	}

	state, err := handler.Update(r.Context(), input.Identifier, []byte(input.PatchDocument))
	if err != nil {
		if IsNotFound(err) {
			writeError(w, "ResourceNotFoundException", err.Error())

			return
		}

		writeError(w, "GeneralServiceException", err.Error())

		return
	}

	ev := ProgressEvent{
		TypeName:        input.TypeName,
		Identifier:      input.Identifier,
		RequestToken:    requestToken(input.ClientToken),
		Operation:       "UPDATE",
		OperationStatus: "SUCCESS",
		EventTime:       nowEpoch(),
		ResourceModel:   string(state),
	}
	s.progress.record(&ev)
	writeJSON(w, ProgressEventOutput{ProgressEvent: ev})
}

// DeleteResource removes the resource. NotFound is reported as
// ResourceNotFoundException, matching real Cloud Control rather than
// silently succeeding.
func (s *Service) DeleteResource(w http.ResponseWriter, r *http.Request) {
	var input DeleteResourceInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, "InvalidRequest", "failed to decode request body: "+err.Error())

		return
	}

	handler, ok := s.registry.Get(input.TypeName)
	if !ok {
		writeError(w, "TypeNotFoundException", "Type "+input.TypeName+" is not supported")

		return
	}

	if err := handler.Delete(r.Context(), input.Identifier); err != nil {
		if IsNotFound(err) {
			writeError(w, "ResourceNotFoundException", err.Error())

			return
		}

		writeError(w, "GeneralServiceException", err.Error())

		return
	}

	ev := ProgressEvent{
		TypeName:        input.TypeName,
		Identifier:      input.Identifier,
		RequestToken:    requestToken(input.ClientToken),
		Operation:       "DELETE",
		OperationStatus: "SUCCESS",
		EventTime:       nowEpoch(),
	}
	s.progress.record(&ev)
	writeJSON(w, ProgressEventOutput{ProgressEvent: ev})
}

// ListResources returns every resource of the given type. Pagination is
// not implemented yet; MaxResults / NextToken are accepted but ignored.
func (s *Service) ListResources(w http.ResponseWriter, r *http.Request) {
	var input ListResourcesInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, "InvalidRequest", "failed to decode request body: "+err.Error())

		return
	}

	handler, ok := s.registry.Get(input.TypeName)
	if !ok {
		writeError(w, "TypeNotFoundException", "Type "+input.TypeName+" is not supported")

		return
	}

	descs, err := handler.List(r.Context())
	if err != nil {
		writeError(w, "GeneralServiceException", err.Error())

		return
	}

	wire := make([]ResourceDescriptionWire, 0, len(descs))
	for _, d := range descs {
		wire = append(wire, ResourceDescriptionWire{
			Identifier: d.Identifier,
			Properties: string(d.Properties),
		})
	}

	writeJSON(w, ListResourcesOutput{
		TypeName:             input.TypeName,
		ResourceDescriptions: wire,
	})
}

// GetResourceRequestStatus is invoked by clients polling an asynchronous
// operation. kumo executes everything synchronously, so we look up and
// re-emit the original ProgressEvent; echoing back its Identifier +
// TypeName is required for the awscc terraform provider's "unknown after
// apply" to resolve.
func (s *Service) GetResourceRequestStatus(w http.ResponseWriter, r *http.Request) {
	var input GetResourceRequestStatusInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, "InvalidRequest", "failed to decode request body: "+err.Error())

		return
	}

	if ev, ok := s.progress.lookup(input.RequestToken); ok {
		writeJSON(w, ProgressEventOutput{ProgressEvent: ev})

		return
	}

	writeJSON(w, ProgressEventOutput{ProgressEvent: ProgressEvent{
		RequestToken:    input.RequestToken,
		OperationStatus: "SUCCESS",
		EventTime:       nowEpoch(),
	}})
}

// requestToken returns clientToken when the caller supplied one,
// otherwise mints a fresh UUID. AWS SDK clients always send a token, but
// the API treats it as optional.
func requestToken(clientToken string) string {
	if clientToken != "" {
		return clientToken
	}

	return uuid.New().String()
}
