package elbv2

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

// Error codes for ELB.
const (
	errInvalidParameter = "InvalidParameterValue"
	errInternalError    = "InternalError"
	errInvalidAction    = "InvalidAction"
)

// CreateLoadBalancer handles the CreateLoadBalancer action.
func (s *Service) CreateLoadBalancer(w http.ResponseWriter, r *http.Request) {
	var req CreateLoadBalancerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeELBError(w, errInvalidParameter, "Name is required", http.StatusBadRequest)

		return
	}

	lb, err := s.storage.CreateLoadBalancer(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateLoadBalancerResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateLoadBalancerResult{
			LoadBalancers: XMLLoadBalancers{
				Members: []XMLLoadBalancer{convertToXMLLoadBalancer(lb)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteLoadBalancer handles the DeleteLoadBalancer action.
func (s *Service) DeleteLoadBalancer(w http.ResponseWriter, r *http.Request) {
	var req DeleteLoadBalancerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LoadBalancerArn == "" {
		writeELBError(w, errInvalidParameter, "LoadBalancerArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteLoadBalancer(r.Context(), req.LoadBalancerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteLoadBalancerResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteLoadBalancerResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeLoadBalancers handles the DescribeLoadBalancers action.
func (s *Service) DescribeLoadBalancers(w http.ResponseWriter, r *http.Request) {
	var req DescribeLoadBalancersRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	lbs, err := s.storage.DescribeLoadBalancers(r.Context(), req.LoadBalancerArns, req.Names)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlLbs := make([]XMLLoadBalancer, 0, len(lbs))
	for _, lb := range lbs {
		xmlLbs = append(xmlLbs, convertToXMLLoadBalancer(lb))
	}

	writeELBXMLResponse(w, XMLDescribeLoadBalancersResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeLoadBalancersResult{
			LoadBalancers: XMLLoadBalancers{Members: xmlLbs},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateTargetGroup handles the CreateTargetGroup action.
func (s *Service) CreateTargetGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateTargetGroupRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeELBError(w, errInvalidParameter, "Name is required", http.StatusBadRequest)

		return
	}

	tg, err := s.storage.CreateTargetGroup(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateTargetGroupResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateTargetGroupResult{
			TargetGroups: XMLTargetGroups{
				Members: []XMLTargetGroup{convertToXMLTargetGroup(tg)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteTargetGroup handles the DeleteTargetGroup action.
func (s *Service) DeleteTargetGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteTargetGroupRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TargetGroupArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteTargetGroup(r.Context(), req.TargetGroupArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteTargetGroupResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteTargetGroupResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeTargetGroups handles the DescribeTargetGroups action.
func (s *Service) DescribeTargetGroups(w http.ResponseWriter, r *http.Request) {
	var req DescribeTargetGroupsRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	tgs, err := s.storage.DescribeTargetGroups(r.Context(), req.TargetGroupArns, req.Names, req.LoadBalancerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlTgs := make([]XMLTargetGroup, 0, len(tgs))
	for _, tg := range tgs {
		xmlTgs = append(xmlTgs, convertToXMLTargetGroup(tg))
	}

	writeELBXMLResponse(w, XMLDescribeTargetGroupsResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeTargetGroupsResult{
			TargetGroups: XMLTargetGroups{Members: xmlTgs},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// RegisterTargets handles the RegisterTargets action. The Query form pattern
// Targets.member.N.{Id,Port,AvailabilityZone} is read directly from r.Form
// because the generic form-to-JSON converter does not understand the nested
// member.N.<field> shape and would emit flat dotted keys instead.
func (s *Service) RegisterTargets(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	tgArn := r.Form.Get("TargetGroupArn")
	if tgArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.RegisterTargets(r.Context(), tgArn, parseELBTargetsFromForm(r.Form)); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLRegisterTargetsResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLRegisterTargetsResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeregisterTargets handles the DeregisterTargets action.
func (s *Service) DeregisterTargets(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	tgArn := r.Form.Get("TargetGroupArn")
	if tgArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeregisterTargets(r.Context(), tgArn, parseELBTargetsFromForm(r.Form)); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeregisterTargetsResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeregisterTargetsResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// parseELBTargetsFromForm reads Targets.member.N.{Id,Port,AvailabilityZone}.
func parseELBTargetsFromForm(form map[string][]string) []Target {
	byIdx := make(map[int]*Target)

	for key, values := range form {
		applyELBTargetFormEntry(byIdx, key, values)
	}

	indexes := make([]int, 0, len(byIdx))
	for n := range byIdx {
		indexes = append(indexes, n)
	}

	sort.Ints(indexes)

	out := make([]Target, 0, len(indexes))
	for _, n := range indexes {
		out = append(out, *byIdx[n])
	}

	return out
}

func applyELBTargetFormEntry(byIdx map[int]*Target, key string, values []string) {
	suffix, ok := strings.CutPrefix(key, "Targets.member.")
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

	entry, exists := byIdx[n]
	if !exists {
		entry = &Target{}
		byIdx[n] = entry
	}

	switch suffix[dot+1:] {
	case "Id":
		entry.ID = values[0]
	case "Port":
		if v, err := strconv.Atoi(values[0]); err == nil {
			entry.Port = v
		}
	case "AvailabilityZone":
		entry.AvailabilityZone = values[0]
	}
}

// CreateListener handles the CreateListener action.
func (s *Service) CreateListener(w http.ResponseWriter, r *http.Request) {
	var req CreateListenerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LoadBalancerArn == "" {
		writeELBError(w, errInvalidParameter, "LoadBalancerArn is required", http.StatusBadRequest)

		return
	}

	listener, err := s.storage.CreateListener(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateListenerResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateListenerResult{
			Listeners: XMLListeners{
				Members: []XMLListener{convertToXMLListener(listener)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteListener handles the DeleteListener action.
func (s *Service) DeleteListener(w http.ResponseWriter, r *http.Request) {
	var req DeleteListenerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ListenerArn == "" {
		writeELBError(w, errInvalidParameter, "ListenerArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteListener(r.Context(), req.ListenerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteListenerResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteListenerResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateRule handles the CreateRule action.
func (s *Service) CreateRule(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	listenerArn := r.Form.Get("ListenerArn")
	priority := r.Form.Get("Priority")

	if listenerArn == "" || priority == "" {
		writeELBError(w, errInvalidParameter, "ListenerArn and Priority are required", http.StatusBadRequest)

		return
	}

	rule, err := s.storage.CreateRule(r.Context(), listenerArn, priority,
		parseRuleConditionsFromForm(r.Form), parseActionsFromForm(r.Form, "Actions"))
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateRuleResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLCreateRuleResult{Rules: XMLRules{Members: []XMLRule{convertRuleToXML(rule)}}},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeRules handles the DescribeRules action.
func (s *Service) DescribeRules(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	listenerArn := r.Form.Get("ListenerArn")
	ruleArns := parseMemberListFromForm(r.Form, "RuleArns")

	rules, err := s.storage.DescribeRules(r.Context(), listenerArn, ruleArns)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDescribeRulesResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDescribeRulesResult{Rules: XMLRules{Members: convertRulesToXML(rules)}},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ModifyRule handles the ModifyRule action.
func (s *Service) ModifyRule(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	ruleArn := r.Form.Get("RuleArn")
	if ruleArn == "" {
		writeELBError(w, errInvalidParameter, "RuleArn is required", http.StatusBadRequest)

		return
	}

	conds := parseRuleConditionsFromForm(r.Form)
	actions := parseActionsFromForm(r.Form, "Actions")

	if len(conds) == 0 {
		conds = nil
	}

	if len(actions) == 0 {
		actions = nil
	}

	rule, err := s.storage.ModifyRule(r.Context(), ruleArn, conds, actions)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLModifyRuleResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLModifyRuleResult{Rules: XMLRules{Members: []XMLRule{convertRuleToXML(rule)}}},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteRule handles the DeleteRule action.
func (s *Service) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	ruleArn := r.Form.Get("RuleArn")
	if ruleArn == "" {
		writeELBError(w, errInvalidParameter, "RuleArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteRule(r.Context(), ruleArn); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteRuleResponse{
		Xmlns:            elbXMLNS,
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// SetRulePriorities handles the SetRulePriorities action.
func (s *Service) SetRulePriorities(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	priorities := parseRulePrioritiesFromForm(r.Form)
	if len(priorities) == 0 {
		writeELBError(w, errInvalidParameter, "RulePriorities is required", http.StatusBadRequest)

		return
	}

	rules, err := s.storage.SetRulePriorities(r.Context(), priorities)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLSetRulePrioritiesResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLSetRulePrioritiesResult{Rules: XMLRules{Members: convertRulesToXML(rules)}},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// parseMemberListFromForm reads <prefix>.member.N entries into a list,
// preserving the index order.
func parseMemberListFromForm(form map[string][]string, prefix string) []string {
	type entry struct {
		idx int
		val string
	}

	entries := make([]entry, 0)

	for key, values := range form {
		suffix, ok := strings.CutPrefix(key, prefix+".member.")
		if !ok || len(values) == 0 {
			continue
		}

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

// parseActionsFromForm reads <prefix>.member.N.{Type,TargetGroupArn,Order}.
func parseActionsFromForm(form map[string][]string, prefix string) []Action {
	byIdx := make(map[int]*Action)

	for key, values := range form {
		suffix, ok := strings.CutPrefix(key, prefix+".member.")
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

		entry, exists := byIdx[n]
		if !exists {
			entry = &Action{}
			byIdx[n] = entry
		}

		switch suffix[dot+1:] {
		case "Type":
			entry.Type = values[0]
		case "TargetGroupArn":
			entry.TargetGroupArn = values[0]
		case "Order":
			if v, err := strconv.Atoi(values[0]); err == nil {
				entry.Order = v
			}
		}
	}

	indexes := make([]int, 0, len(byIdx))
	for n := range byIdx {
		indexes = append(indexes, n)
	}

	sort.Ints(indexes)

	out := make([]Action, 0, len(indexes))
	for _, n := range indexes {
		out = append(out, *byIdx[n])
	}

	return out
}

// ruleConditionAcc accumulates one Conditions.member.N entry being parsed.
type ruleConditionAcc struct {
	field  string
	values map[int]string
}

// parseRuleConditionsFromForm reads Conditions.member.N.Field and either
// Conditions.member.N.Values.member.M (legacy) or
// Conditions.member.N.<Field>Config.Values.member.M (modern). Both are
// stored as Field + Values.
func parseRuleConditionsFromForm(form map[string][]string) []RuleCondition {
	byIdx := make(map[int]*ruleConditionAcc)

	for key, values := range form {
		suffix, ok := strings.CutPrefix(key, "Conditions.member.")
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

		entry, exists := byIdx[n]
		if !exists {
			entry = &ruleConditionAcc{values: make(map[int]string)}
			byIdx[n] = entry
		}

		applyRuleConditionField(entry, suffix[dot+1:], values[0])
	}

	indexes := make([]int, 0, len(byIdx))
	for n := range byIdx {
		indexes = append(indexes, n)
	}

	sort.Ints(indexes)

	out := make([]RuleCondition, 0, len(indexes))

	for _, n := range indexes {
		entry := byIdx[n]
		vals := flattenValuesMap(entry.values)
		out = append(out, RuleCondition{Field: entry.field, Values: vals})
	}

	return out
}

func applyRuleConditionField(entry *ruleConditionAcc, field, value string) {
	switch {
	case field == "Field":
		entry.field = value
	case strings.HasPrefix(field, "Values.member."):
		recordIndexedValue(entry.values, strings.TrimPrefix(field, "Values.member."), value)
	default:
		// Modern <Field>Config.Values.member.N pattern.
		if i := strings.Index(field, "Config.Values.member."); i > 0 {
			recordIndexedValue(entry.values, field[i+len("Config.Values.member."):], value)
		}
	}
}

func recordIndexedValue(m map[int]string, suffix, value string) {
	n, err := strconv.Atoi(suffix)
	if err != nil {
		return
	}

	m[n] = value
}

func flattenValuesMap(m map[int]string) []string {
	indexes := make([]int, 0, len(m))
	for n := range m {
		indexes = append(indexes, n)
	}

	sort.Ints(indexes)

	out := make([]string, 0, len(indexes))
	for _, n := range indexes {
		out = append(out, m[n])
	}

	return out
}

// parseRulePrioritiesFromForm reads RulePriorities.member.N.{RuleArn,Priority}.
func parseRulePrioritiesFromForm(form map[string][]string) map[string]string {
	type entry struct {
		arn      string
		priority string
	}

	byIdx := make(map[int]*entry)

	for key, values := range form {
		suffix, ok := strings.CutPrefix(key, "RulePriorities.member.")
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

		ent, exists := byIdx[n]
		if !exists {
			ent = &entry{}
			byIdx[n] = ent
		}

		switch suffix[dot+1:] {
		case "RuleArn":
			ent.arn = values[0]
		case "Priority":
			ent.priority = values[0]
		}
	}

	out := make(map[string]string)

	for _, ent := range byIdx {
		if ent.arn != "" {
			out[ent.arn] = ent.priority
		}
	}

	return out
}

// convertRuleToXML converts a Rule to its XML form.
func convertRuleToXML(rule *Rule) XMLRule {
	conds := make([]XMLRuleCondition, 0, len(rule.Conditions))
	for _, c := range rule.Conditions {
		conds = append(conds, XMLRuleCondition{
			Field:  c.Field,
			Values: XMLRuleValues{Members: append([]string(nil), c.Values...)},
		})
	}

	actions := make([]XMLAction, 0, len(rule.Actions))
	for _, a := range rule.Actions {
		actions = append(actions, XMLAction(a))
	}

	return XMLRule{
		RuleArn:    rule.RuleArn,
		Priority:   rule.Priority,
		Conditions: XMLRuleConditions{Members: conds},
		Actions:    XMLActions{Members: actions},
		IsDefault:  rule.IsDefault,
	}
}

func convertRulesToXML(rules []Rule) []XMLRule {
	out := make([]XMLRule, 0, len(rules))
	for i := range rules {
		out = append(out, convertRuleToXML(&rules[i]))
	}

	return out
}

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)
	handler := s.getActionHandler(action)

	if handler == nil {
		writeELBError(w, errInvalidAction, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// getActionHandler returns the handler function for the given action.
func (s *Service) getActionHandler(action string) func(http.ResponseWriter, *http.Request) {
	handlers := map[string]func(http.ResponseWriter, *http.Request){
		"CreateLoadBalancer":             s.CreateLoadBalancer,
		"DeleteLoadBalancer":             s.DeleteLoadBalancer,
		"DescribeLoadBalancers":          s.DescribeLoadBalancers,
		"CreateTargetGroup":              s.CreateTargetGroup,
		"DeleteTargetGroup":              s.DeleteTargetGroup,
		"DescribeTargetGroups":           s.DescribeTargetGroups,
		"RegisterTargets":                s.RegisterTargets,
		"DeregisterTargets":              s.DeregisterTargets,
		"CreateListener":                 s.CreateListener,
		"DeleteListener":                 s.DeleteListener,
		"CreateRule":                     s.CreateRule,
		"DescribeRules":                  s.DescribeRules,
		"ModifyRule":                     s.ModifyRule,
		"DeleteRule":                     s.DeleteRule,
		"SetRulePriorities":              s.SetRulePriorities,
		"ModifyLoadBalancerAttributes":   s.ModifyLoadBalancerAttributes,
		"DescribeLoadBalancerAttributes": s.DescribeLoadBalancerAttributes,
		"ModifyTargetGroupAttributes":    s.ModifyTargetGroupAttributes,
		"DescribeTargetGroupAttributes":  s.DescribeTargetGroupAttributes,
		"DescribeListeners":              s.DescribeListeners,
		"ModifyListener":                 s.ModifyListener,
		"DescribeTargetHealth":           s.DescribeTargetHealth,
	}

	return handlers[action]
}

// Helper functions.

// convertToXMLLoadBalancer converts a LoadBalancer to XMLLoadBalancer.
func convertToXMLLoadBalancer(lb *LoadBalancer) XMLLoadBalancer {
	azs := make([]XMLAvailabilityZone, 0, len(lb.AvailabilityZones))
	for _, az := range lb.AvailabilityZones {
		azs = append(azs, XMLAvailabilityZone{
			ZoneName: az.ZoneName,
			SubnetID: az.SubnetID,
		})
	}

	return XMLLoadBalancer{
		LoadBalancerArn:       lb.LoadBalancerArn,
		DNSName:               lb.DNSName,
		CanonicalHostedZoneID: lb.CanonicalHostedZoneID,
		CreatedTime:           lb.CreatedTime.Format("2006-01-02T15:04:05.000Z"),
		LoadBalancerName:      lb.LoadBalancerName,
		Scheme:                lb.Scheme,
		VpcID:                 lb.VpcID,
		State:                 XMLLoadBalancerState{Code: lb.State.Code, Reason: lb.State.Reason},
		Type:                  lb.Type,
		AvailabilityZones:     XMLAvailabilityZones{Members: azs},
		SecurityGroups:        XMLSecurityGroups{Members: lb.SecurityGroups},
		IPAddressType:         lb.IPAddressType,
	}
}

// convertToXMLTargetGroup converts a TargetGroup to XMLTargetGroup.
func convertToXMLTargetGroup(tg *TargetGroup) XMLTargetGroup {
	return XMLTargetGroup{
		TargetGroupArn:             tg.TargetGroupArn,
		TargetGroupName:            tg.TargetGroupName,
		Protocol:                   tg.Protocol,
		Port:                       tg.Port,
		VpcID:                      tg.VpcID,
		HealthCheckEnabled:         tg.HealthCheckEnabled,
		HealthCheckIntervalSeconds: tg.HealthCheckIntervalSeconds,
		HealthCheckPath:            tg.HealthCheckPath,
		HealthCheckPort:            tg.HealthCheckPort,
		HealthCheckProtocol:        tg.HealthCheckProtocol,
		HealthCheckTimeoutSeconds:  tg.HealthCheckTimeoutSeconds,
		HealthyThresholdCount:      tg.HealthyThresholdCount,
		UnhealthyThresholdCount:    tg.UnhealthyThresholdCount,
		TargetType:                 tg.TargetType,
		LoadBalancerArns:           XMLLoadBalancerArns{Members: tg.LoadBalancerArns},
	}
}

// convertToXMLListener converts a Listener to XMLListener.
func convertToXMLListener(l *Listener) XMLListener {
	actions := make([]XMLAction, 0, len(l.DefaultActions))
	for _, a := range l.DefaultActions {
		actions = append(actions, XMLAction(a))
	}

	return XMLListener{
		ListenerArn:     l.ListenerArn,
		LoadBalancerArn: l.LoadBalancerArn,
		Port:            l.Port,
		Protocol:        l.Protocol,
		DefaultActions:  XMLActions{Members: actions},
	}
}

// readELBJSONRequest reads and decodes JSON request body.
func readELBJSONRequest(r *http.Request, v any) error {
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
func extractAction(r *http.Request) string {
	// Try X-Amz-Target header (format: "ElasticLoadBalancing.ActionName").
	target := r.Header.Get("X-Amz-Target")
	if target != "" {
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			return target[idx+1:]
		}
	}

	// Fallback to URL query parameter.
	return r.URL.Query().Get("Action")
}

// writeELBXMLResponse writes an XML response with HTTP 200 OK.
func writeELBXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// writeELBError writes an ELB error response in XML format.
func writeELBError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Error: XMLError{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		RequestID: requestID,
	})
}

// handleELBError handles ELB errors and writes the appropriate response.
func handleELBError(w http.ResponseWriter, err error) {
	var elbErr *Error
	if errors.As(err, &elbErr) {
		writeELBError(w, elbErr.Code, elbErr.Message, http.StatusBadRequest)

		return
	}

	writeELBError(w, errInternalError, "Internal server error", http.StatusInternalServerError)
}

// ModifyLoadBalancerAttributes handles the ModifyLoadBalancerAttributes action.
func (s *Service) ModifyLoadBalancerAttributes(w http.ResponseWriter, r *http.Request) {
	lbArn, attrs, err := readAttributesRequestForm(r, "LoadBalancerArn")
	if err != nil {
		writeELBError(w, errInvalidParameter, err.Error(), http.StatusBadRequest)

		return
	}

	updated, err := s.storage.ModifyLoadBalancerAttributes(r.Context(), lbArn, attrs)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLModifyLoadBalancerAttributesResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLModifyLoadBalancerAttributesResult{Attributes: attributesToXML(updated)},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeLoadBalancerAttributes handles the DescribeLoadBalancerAttributes action.
func (s *Service) DescribeLoadBalancerAttributes(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	lbArn := r.Form.Get("LoadBalancerArn")
	if lbArn == "" {
		writeELBError(w, errInvalidParameter, "LoadBalancerArn is required", http.StatusBadRequest)

		return
	}

	attrs, err := s.storage.DescribeLoadBalancerAttributes(r.Context(), lbArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDescribeLoadBalancerAttributesResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDescribeLoadBalancerAttributesResult{Attributes: attributesToXML(attrs)},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ModifyTargetGroupAttributes handles the ModifyTargetGroupAttributes action.
func (s *Service) ModifyTargetGroupAttributes(w http.ResponseWriter, r *http.Request) {
	tgArn, attrs, err := readAttributesRequestForm(r, "TargetGroupArn")
	if err != nil {
		writeELBError(w, errInvalidParameter, err.Error(), http.StatusBadRequest)

		return
	}

	updated, err := s.storage.ModifyTargetGroupAttributes(r.Context(), tgArn, attrs)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLModifyTargetGroupAttributesResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLModifyTargetGroupAttributesResult{Attributes: attributesToXML(updated)},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeTargetGroupAttributes handles the DescribeTargetGroupAttributes action.
func (s *Service) DescribeTargetGroupAttributes(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	tgArn := r.Form.Get("TargetGroupArn")
	if tgArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	attrs, err := s.storage.DescribeTargetGroupAttributes(r.Context(), tgArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDescribeTargetGroupAttributesResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDescribeTargetGroupAttributesResult{Attributes: attributesToXML(attrs)},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// readAttributesRequestForm extracts the resource ARN and the
// Attributes.member.N.{Key,Value} pairs from the AWS Query form.
func readAttributesRequestForm(r *http.Request, arnField string) (string, map[string]string, error) {
	if err := r.ParseForm(); err != nil {
		return "", nil, fmt.Errorf("failed to parse form: %w", err)
	}

	arn := r.Form.Get(arnField)
	if arn == "" {
		return "", nil, fmt.Errorf("%s is required", arnField)
	}

	return arn, parseAttributePairsFromForm(r.Form), nil
}

// attributePairAcc accumulates one Attributes.member.N pair being parsed.
type attributePairAcc struct {
	key   string
	value string
}

// parseAttributePairsFromForm reads Attributes.member.N.Key/Value into a map.
func parseAttributePairsFromForm(form map[string][]string) map[string]string {
	byIdx := make(map[int]*attributePairAcc)

	for key, values := range form {
		applyAttributePairFormEntry(byIdx, key, values)
	}

	out := make(map[string]string)

	for _, entry := range byIdx {
		if entry.key != "" {
			out[entry.key] = entry.value
		}
	}

	return out
}

func applyAttributePairFormEntry(byIdx map[int]*attributePairAcc, key string, values []string) {
	suffix, ok := strings.CutPrefix(key, "Attributes.member.")
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

	entry, exists := byIdx[n]
	if !exists {
		entry = &attributePairAcc{}
		byIdx[n] = entry
	}

	switch suffix[dot+1:] {
	case "Key":
		entry.key = values[0]
	case "Value":
		entry.value = values[0]
	}
}

// attributesToXML converts a name->value map to the XML wire shape, sorted
// by key for deterministic output.
func attributesToXML(attrs map[string]string) XMLAttributePairs {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	members := make([]XMLAttributePair, 0, len(keys))
	for _, k := range keys {
		members = append(members, XMLAttributePair{Key: k, Value: attrs[k]})
	}

	return XMLAttributePairs{Members: members}
}

// DescribeListeners handles the DescribeListeners action.
func (s *Service) DescribeListeners(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	listenerArns := parseELBMemberListFromForm(r.Form, "ListenerArns")
	lbArn := r.Form.Get("LoadBalancerArn")

	listeners, err := s.storage.DescribeListeners(r.Context(), listenerArns, lbArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlListeners := make([]XMLListener, 0, len(listeners))
	for _, l := range listeners {
		xmlListeners = append(xmlListeners, convertToXMLListener(l))
	}

	writeELBXMLResponse(w, XMLDescribeListenersResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDescribeListenersResult{Listeners: XMLListeners{Members: xmlListeners}},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ModifyListener handles the ModifyListener action.
func (s *Service) ModifyListener(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	listenerArn := r.Form.Get("ListenerArn")
	if listenerArn == "" {
		writeELBError(w, errInvalidParameter, "ListenerArn is required", http.StatusBadRequest)

		return
	}

	port := 0

	if p := r.Form.Get("Port"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	protocol := r.Form.Get("Protocol")

	defaultActions := parseELBActionsFromForm(r.Form, "DefaultActions")
	if len(defaultActions) == 0 {
		defaultActions = nil
	}

	listener, err := s.storage.ModifyListener(r.Context(), listenerArn, port, protocol, defaultActions)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLModifyListenerResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLModifyListenerResult{Listeners: XMLListeners{Members: []XMLListener{convertToXMLListener(listener)}}},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeTargetHealth handles the DescribeTargetHealth action.
func (s *Service) DescribeTargetHealth(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	tgArn := r.Form.Get("TargetGroupArn")
	if tgArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	targets := parseELBTargetsFromForm(r.Form)

	descriptions, err := s.storage.DescribeTargetHealth(r.Context(), tgArn, targets)
	if err != nil {
		handleELBError(w, err)

		return
	}

	members := make([]XMLTargetHealthDescription, 0, len(descriptions))
	for _, d := range descriptions {
		members = append(members, XMLTargetHealthDescription{
			Target: XMLTargetForHealth{
				ID:               d.Target.ID,
				Port:             d.Target.Port,
				AvailabilityZone: d.Target.AvailabilityZone,
			},
			TargetHealth: XMLTargetHealth{State: d.HealthState},
		})
	}

	writeELBXMLResponse(w, XMLDescribeTargetHealthResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDescribeTargetHealthResult{TargetHealthDescriptions: XMLTargetHealthDescriptions{Members: members}},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// parseELBMemberListFromForm reads <prefix>.member.N entries.
func parseELBMemberListFromForm(form map[string][]string, prefix string) []string {
	type entry struct {
		idx int
		val string
	}

	entries := make([]entry, 0)

	for key, values := range form {
		suffix, ok := strings.CutPrefix(key, prefix+".member.")
		if !ok || len(values) == 0 {
			continue
		}

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

// parseELBActionsFromForm reads <prefix>.member.N.{Type,TargetGroupArn,Order}.
func parseELBActionsFromForm(form map[string][]string, prefix string) []Action {
	byIdx := make(map[int]*Action)

	for key, values := range form {
		applyELBActionFormEntry(byIdx, prefix, key, values)
	}

	indexes := make([]int, 0, len(byIdx))
	for n := range byIdx {
		indexes = append(indexes, n)
	}

	sort.Ints(indexes)

	out := make([]Action, 0, len(indexes))
	for _, n := range indexes {
		out = append(out, *byIdx[n])
	}

	return out
}

func applyELBActionFormEntry(byIdx map[int]*Action, prefix, key string, values []string) {
	suffix, ok := strings.CutPrefix(key, prefix+".member.")
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

	entry, exists := byIdx[n]
	if !exists {
		entry = &Action{}
		byIdx[n] = entry
	}

	switch suffix[dot+1:] {
	case "Type":
		entry.Type = values[0]
	case "TargetGroupArn":
		entry.TargetGroupArn = values[0]
	case "Order":
		if v, err := strconv.Atoi(values[0]); err == nil {
			entry.Order = v
		}
	}
}
