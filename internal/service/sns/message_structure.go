package sns

import (
	"encoding/json"
)

// MessageStructure=json handling. See the "JSON-specific constraints" of
// https://docs.aws.amazon.com/sns/latest/api/API_Publish.html
const (
	messageStructureJSON = "json"
	protocolDefault      = "default"

	msgStructureNoDefault   = "Invalid parameter: Message Structure - No default entry in JSON message body"
	msgStructureParseFailed = "Invalid parameter: Message Structure - JSON message body failed to parse"
)

// protocolMessages holds the per-protocol message bodies of one Publish.
// A plain Publish has only the default entry.
type protocolMessages map[string]string

// forProtocol returns the entry for protocol when the body defined one
// (an explicit empty string counts) and the default entry otherwise.
func (p protocolMessages) forProtocol(protocol string) string {
	if message, ok := p[protocol]; ok {
		return message
	}

	return p[protocolDefault]
}

// messagesForStructure interprets the Publish Message parameter according to
// MessageStructure: verbatim for a plain message, parsed and validated for json.
func messagesForStructure(message, messageStructure string) (protocolMessages, error) {
	if messageStructure != messageStructureJSON {
		return protocolMessages{protocolDefault: message}, nil
	}

	return parseJSONMessageStructure(message)
}

// parseJSONMessageStructure applies AWS's rules for a json-structured body:
// the body must be a JSON object, only keys naming a transport protocol are
// kept, a non-string value drops its key, and a default entry is required.
// Duplicate keys are accepted (the service does, despite its docs) and the
// last value wins.
func parseJSONMessageStructure(body string) (protocolMessages, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &object); err != nil || object == nil {
		return nil, &TopicError{Code: errInvalidParameter, Message: msgStructureParseFailed}
	}

	messages := make(protocolMessages, len(object))

	for key, raw := range object {
		if !isStructuredMessageKey(key) {
			continue
		}

		var value string
		if json.Unmarshal(raw, &value) != nil {
			continue
		}

		messages[key] = value
	}

	if _, ok := messages[protocolDefault]; !ok {
		return nil, &TopicError{Code: errInvalidParameter, Message: msgStructureNoDefault}
	}

	return messages, nil
}

// isStructuredMessageKey reports whether key is one of the transport
// protocol keys SNS recognises in a json-structured message body.
func isStructuredMessageKey(key string) bool {
	switch key {
	case protocolDefault, protocolSQS, protocolLambda, protocolHTTP, protocolHTTPS, "email", "email-json", "sms", "application", "firehose":
		return true
	default:
		return false
	}
}
