package comprehend

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	errInvalidRequest        = "InvalidRequestException"
	errTextSizeLimitExceeded = "TextSizeLimitExceededException"
)

// Analyzer provides NLP analysis capabilities.
type Analyzer struct{}

// NewAnalyzer creates a new analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// DetectSentiment analyzes the sentiment of the given text.
func (a *Analyzer) DetectSentiment(text, _ string) (*DetectSentimentResponse, error) {
	if err := a.validateText(text); err != nil {
		return nil, err
	}

	sentiment, score := a.analyzeSentiment(text)

	return &DetectSentimentResponse{
		Sentiment:      sentiment,
		SentimentScore: score,
	}, nil
}

// DetectDominantLanguage detects the dominant language of the given text.
func (a *Analyzer) DetectDominantLanguage(text string) (*DetectDominantLanguageResponse, error) {
	if err := a.validateText(text); err != nil {
		return nil, err
	}

	// For the emulator, we default to English with high confidence.
	languages := []DominantLanguage{
		{
			LanguageCode: "en",
			Score:        0.99,
		},
	}

	return &DetectDominantLanguageResponse{
		Languages: languages,
	}, nil
}

// DetectEntities detects named entities in the given text.
func (a *Analyzer) DetectEntities(text, _ string) (*DetectEntitiesResponse, error) {
	if err := a.validateText(text); err != nil {
		return nil, err
	}

	entities := a.extractEntities(text)

	return &DetectEntitiesResponse{
		Entities: entities,
	}, nil
}

// DetectKeyPhrases detects key phrases in the given text.
func (a *Analyzer) DetectKeyPhrases(text, _ string) (*DetectKeyPhrasesResponse, error) {
	if err := a.validateText(text); err != nil {
		return nil, err
	}

	keyPhrases := a.extractKeyPhrases(text)

	return &DetectKeyPhrasesResponse{
		KeyPhrases: keyPhrases,
	}, nil
}

// DetectPiiEntities detects PII entities in the given text.
func (a *Analyzer) DetectPiiEntities(text, _ string) (*DetectPiiEntitiesResponse, error) {
	if err := a.validateText(text); err != nil {
		return nil, err
	}

	piiEntities := a.extractPiiEntities(text)

	return &DetectPiiEntitiesResponse{
		Entities: piiEntities,
	}, nil
}

// DetectSyntax analyzes the syntax of the given text.
func (a *Analyzer) DetectSyntax(text, _ string) (*DetectSyntaxResponse, error) {
	if err := a.validateText(text); err != nil {
		return nil, err
	}

	tokens := a.tokenize(text)

	return &DetectSyntaxResponse{
		SyntaxTokens: tokens,
	}, nil
}

// ContainsPiiEntities checks if the text contains PII entities.
func (a *Analyzer) ContainsPiiEntities(text, _ string) (*ContainsPiiEntitiesResponse, error) {
	if err := a.validateText(text); err != nil {
		return nil, err
	}

	labels := a.detectPiiLabels(text)

	return &ContainsPiiEntitiesResponse{
		Labels: labels,
	}, nil
}

// validateText validates the input text.
func (a *Analyzer) validateText(text string) error {
	if text == "" {
		return &Error{
			Code:    errInvalidRequest,
			Message: "1 validation error detected: Value at 'text' failed to satisfy constraint: Member must not be null",
		}
	}

	// AWS Comprehend has a 100KB limit for most operations.
	if len(text) > 100*1024 {
		return &Error{
			Code:    errTextSizeLimitExceeded,
			Message: "Input text size exceeds limit. Max length of text is 100000 bytes",
		}
	}

	return nil
}

// analyzeSentiment performs basic sentiment analysis.
func (a *Analyzer) analyzeSentiment(text string) (SentimentType, *SentimentScore) {
	lowerText := strings.ToLower(text)

	positiveWords := []string{"good", "great", "excellent", "amazing", "wonderful", "fantastic", "love", "happy", "best", "awesome", "perfect", "nice", "beautiful", "positive"}
	negativeWords := []string{"bad", "terrible", "horrible", "awful", "worst", "hate", "sad", "angry", "poor", "negative", "wrong", "fail", "error", "problem"}

	positiveCount := 0
	negativeCount := 0

	for _, word := range positiveWords {
		positiveCount += strings.Count(lowerText, word)
	}

	for _, word := range negativeWords {
		negativeCount += strings.Count(lowerText, word)
	}

	var sentiment SentimentType

	var score SentimentScore

	switch {
	case positiveCount > negativeCount:
		sentiment = SentimentPositive
		score = SentimentScore{
			Positive: 0.85,
			Negative: 0.05,
			Neutral:  0.08,
			Mixed:    0.02,
		}
	case negativeCount > positiveCount:
		sentiment = SentimentNegative
		score = SentimentScore{
			Positive: 0.05,
			Negative: 0.85,
			Neutral:  0.08,
			Mixed:    0.02,
		}
	case positiveCount > 0 && negativeCount > 0:
		sentiment = SentimentMixed
		score = SentimentScore{
			Positive: 0.35,
			Negative: 0.35,
			Neutral:  0.10,
			Mixed:    0.20,
		}
	default:
		sentiment = SentimentNeutral
		score = SentimentScore{
			Positive: 0.10,
			Negative: 0.10,
			Neutral:  0.75,
			Mixed:    0.05,
		}
	}

	return sentiment, &score
}

// extractEntities extracts named entities from text using simple patterns.
func (a *Analyzer) extractEntities(text string) []Entity {
	entities := []Entity{}

	// Detect capitalized words as potential entities (simplified approach).
	words := strings.Fields(text)
	currentOffset := 0

	for _, word := range words {
		idx := strings.Index(text[currentOffset:], word)
		if idx == -1 {
			continue
		}

		beginOffset := currentOffset + idx
		endOffset := beginOffset + len(word)
		currentOffset = endOffset

		cleanWord := strings.Trim(word, ".,!?;:'\"")

		if len(cleanWord) > 1 && unicode.IsUpper(rune(cleanWord[0])) && !isCommonWord(cleanWord) {
			entityType := guessEntityType(cleanWord)
			entities = append(entities, Entity{
				BeginOffset: beginOffset,
				EndOffset:   beginOffset + len(cleanWord),
				Score:       0.95,
				Text:        cleanWord,
				Type:        entityType,
			})
		}
	}

	return entities
}

// extractKeyPhrases extracts key phrases from text.
func (a *Analyzer) extractKeyPhrases(text string) []KeyPhrase {
	keyPhrases := []KeyPhrase{}

	// Simple approach: extract noun phrases (sequences of capitalized words or quoted text).
	quotePattern := regexp.MustCompile(`"([^"]+)"`)
	matches := quotePattern.FindAllStringSubmatchIndex(text, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			phrase := text[match[2]:match[3]]
			keyPhrases = append(keyPhrases, KeyPhrase{
				BeginOffset: match[2],
				EndOffset:   match[3],
				Score:       0.90,
				Text:        phrase,
			})
		}
	}

	capPattern := regexp.MustCompile(`\b([A-Z][a-z]+(?:\s+[A-Z][a-z]+)+)\b`)
	capMatches := capPattern.FindAllStringSubmatchIndex(text, -1)

	for _, match := range capMatches {
		if len(match) >= 4 {
			phrase := text[match[2]:match[3]]
			keyPhrases = append(keyPhrases, KeyPhrase{
				BeginOffset: match[2],
				EndOffset:   match[3],
				Score:       0.85,
				Text:        phrase,
			})
		}
	}

	return keyPhrases
}

// extractPiiEntities extracts PII entities from text.
func (a *Analyzer) extractPiiEntities(text string) []PiiEntity {
	piiEntities := make([]PiiEntity, 0) //nolint:prealloc // Capacity unknown as we check multiple patterns.

	emailPattern := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	emailMatches := emailPattern.FindAllStringIndex(text, -1)

	for _, match := range emailMatches {
		piiEntities = append(piiEntities, PiiEntity{
			BeginOffset: match[0],
			EndOffset:   match[1],
			Score:       0.99,
			Type:        PiiEntityTypeEmail,
		})
	}

	phonePattern := regexp.MustCompile(`\b(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)
	phoneMatches := phonePattern.FindAllStringIndex(text, -1)

	for _, match := range phoneMatches {
		piiEntities = append(piiEntities, PiiEntity{
			BeginOffset: match[0],
			EndOffset:   match[1],
			Score:       0.95,
			Type:        PiiEntityTypePhone,
		})
	}

	ssnPattern := regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	ssnMatches := ssnPattern.FindAllStringIndex(text, -1)

	for _, match := range ssnMatches {
		piiEntities = append(piiEntities, PiiEntity{
			BeginOffset: match[0],
			EndOffset:   match[1],
			Score:       0.99,
			Type:        PiiEntityTypeSSN,
		})
	}

	ccPattern := regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`)
	ccMatches := ccPattern.FindAllStringIndex(text, -1)

	for _, match := range ccMatches {
		piiEntities = append(piiEntities, PiiEntity{
			BeginOffset: match[0],
			EndOffset:   match[1],
			Score:       0.95,
			Type:        PiiEntityTypeCreditCard,
		})
	}

	return piiEntities
}

// tokenize performs basic tokenization.
func (a *Analyzer) tokenize(text string) []SyntaxToken {
	tokens := []SyntaxToken{}
	words := strings.Fields(text)
	currentOffset := 0
	tokenID := 1

	for _, word := range words {
		idx := strings.Index(text[currentOffset:], word)
		if idx == -1 {
			continue
		}

		beginOffset := currentOffset + idx
		endOffset := beginOffset + len(word)
		currentOffset = endOffset

		cleanWord := strings.Trim(word, ".,!?;:'\"")
		tag := guessPartOfSpeech(cleanWord)

		tokens = append(tokens, SyntaxToken{
			BeginOffset: beginOffset,
			EndOffset:   endOffset,
			PartOfSpeech: &PartOfSpeech{
				Score: 0.95,
				Tag:   tag,
			},
			Text:    word,
			TokenID: tokenID,
		})
		tokenID++
	}

	return tokens
}

// detectPiiLabels detects PII labels in text.
func (a *Analyzer) detectPiiLabels(text string) []EntityLabel {
	labels := []EntityLabel{}

	emailPattern := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	if emailPattern.MatchString(text) {
		labels = append(labels, EntityLabel{
			Name:  PiiEntityTypeEmail,
			Score: 0.99,
		})
	}

	phonePattern := regexp.MustCompile(`\b(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)
	if phonePattern.MatchString(text) {
		labels = append(labels, EntityLabel{
			Name:  PiiEntityTypePhone,
			Score: 0.95,
		})
	}

	ssnPattern := regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	if ssnPattern.MatchString(text) {
		labels = append(labels, EntityLabel{
			Name:  PiiEntityTypeSSN,
			Score: 0.99,
		})
	}

	return labels
}

// isCommonWord checks if a word is a common word that shouldn't be treated as an entity.
func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"The": true, "A": true, "An": true, "This": true, "That": true,
		"It": true, "I": true, "We": true, "They": true, "He": true,
		"She": true, "You": true, "My": true, "Your": true, "His": true,
		"Her": true, "Its": true, "Our": true, "Their": true,
	}

	return commonWords[word]
}

// guessEntityType guesses the entity type based on simple heuristics.
func guessEntityType(word string) EntityType {
	locationSuffixes := []string{"City", "State", "Country", "Island", "Mountain", "River", "Lake", "Street", "Avenue", "Road"}

	for _, suffix := range locationSuffixes {
		if strings.HasSuffix(word, suffix) {
			return EntityTypeLocation
		}
	}

	orgSuffixes := []string{"Inc", "Corp", "LLC", "Ltd", "Company", "Organization", "Foundation", "Institute", "University", "College"}

	for _, suffix := range orgSuffixes {
		if strings.HasSuffix(word, suffix) || strings.Contains(word, suffix) {
			return EntityTypeOrganization
		}
	}

	return EntityTypePerson
}

// guessPartOfSpeech guesses the part of speech for a word.
// Package-level maps for part of speech detection.
var (
	posWordMap = map[string]SyntaxTokenType{
		// Determiners
		"the": SyntaxTokenDet, "a": SyntaxTokenDet, "an": SyntaxTokenDet, "this": SyntaxTokenDet, "that": SyntaxTokenDet, "these": SyntaxTokenDet, "those": SyntaxTokenDet,
		// Pronouns
		"i": SyntaxTokenPron, "you": SyntaxTokenPron, "he": SyntaxTokenPron, "she": SyntaxTokenPron, "it": SyntaxTokenPron, "we": SyntaxTokenPron, "they": SyntaxTokenPron, "me": SyntaxTokenPron, "him": SyntaxTokenPron, "her": SyntaxTokenPron, "us": SyntaxTokenPron, "them": SyntaxTokenPron,
		// Prepositions
		"in": SyntaxTokenAdp, "on": SyntaxTokenAdp, "at": SyntaxTokenAdp, "by": SyntaxTokenAdp, "for": SyntaxTokenAdp, "with": SyntaxTokenAdp, "about": SyntaxTokenAdp, "to": SyntaxTokenAdp, "from": SyntaxTokenAdp, "of": SyntaxTokenAdp,
		// Conjunctions
		"and": SyntaxTokenCconj, "or": SyntaxTokenCconj, "but": SyntaxTokenCconj, "nor": SyntaxTokenCconj, "yet": SyntaxTokenCconj, "so": SyntaxTokenCconj,
		// Common verbs
		"is": SyntaxTokenVerb, "are": SyntaxTokenVerb, "was": SyntaxTokenVerb, "were": SyntaxTokenVerb, "be": SyntaxTokenVerb, "been": SyntaxTokenVerb, "being": SyntaxTokenVerb, "have": SyntaxTokenVerb, "has": SyntaxTokenVerb, "had": SyntaxTokenVerb, "do": SyntaxTokenVerb, "does": SyntaxTokenVerb, "did": SyntaxTokenVerb,
	}

	verbSuffixes = []string{"ing", "ed", "ize"}
	adjSuffixes  = []string{"ful", "ous", "ive", "able"}
)

func guessPartOfSpeech(word string) SyntaxTokenType {
	lowerWord := strings.ToLower(word)

	if pos, ok := posWordMap[lowerWord]; ok {
		return pos
	}

	if len(word) == 1 && strings.ContainsAny(word, ".,!?;:'\"") {
		return SyntaxTokenPunct
	}

	if word != "" && unicode.IsUpper(rune(word[0])) {
		return SyntaxTokenPropn
	}

	if hasSuffix(lowerWord, verbSuffixes) {
		return SyntaxTokenVerb
	}

	if strings.HasSuffix(lowerWord, "ly") {
		return SyntaxTokenAdv
	}

	if hasSuffix(lowerWord, adjSuffixes) {
		return SyntaxTokenAdj
	}

	return SyntaxTokenNoun
}

// hasSuffix checks if the word has any of the given suffixes.
func hasSuffix(word string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(word, suffix) {
			return true
		}
	}

	return false
}
