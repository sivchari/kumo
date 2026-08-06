//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/comprehend"
	"github.com/aws/aws-sdk-go-v2/service/comprehend/types"
	"github.com/sivchari/golden"
)

func newComprehendClient(t *testing.T) *comprehend.Client {
	t.Helper()

	return comprehend.NewFromConfig(awsConfig(t), func(o *comprehend.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestComprehend_DetectSentiment(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	// Test positive sentiment.
	positiveOutput, err := client.DetectSentiment(ctx, &comprehend.DetectSentimentInput{
		LanguageCode: types.LanguageCodeEn,
		Text:         aws.String("I love this product! It is absolutely amazing and wonderful."),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("positive", positiveOutput)

	// Test negative sentiment.
	negativeOutput, err := client.DetectSentiment(ctx, &comprehend.DetectSentimentInput{
		LanguageCode: types.LanguageCodeEn,
		Text:         aws.String("This is terrible. I hate it and it is the worst thing ever."),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g2.Assert("negative", negativeOutput)

	// Test neutral sentiment.
	neutralOutput, err := client.DetectSentiment(ctx, &comprehend.DetectSentimentInput{
		LanguageCode: types.LanguageCodeEn,
		Text:         aws.String("The meeting is scheduled for tomorrow at 3 PM."),
	})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g3.Assert("neutral", neutralOutput)
}

func TestComprehend_DetectDominantLanguage(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	output, err := client.DetectDominantLanguage(ctx, &comprehend.DetectDominantLanguageInput{
		Text: aws.String("Hello, how are you doing today?"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("detect", output)
}

func TestComprehend_DetectEntities(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	output, err := client.DetectEntities(ctx, &comprehend.DetectEntitiesInput{
		LanguageCode: types.LanguageCodeEn,
		Text:         aws.String("John Smith works at Amazon in Seattle."),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("detect", output)
}

func TestComprehend_DetectKeyPhrases(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	output, err := client.DetectKeyPhrases(ctx, &comprehend.DetectKeyPhrasesInput{
		LanguageCode: types.LanguageCodeEn,
		Text:         aws.String("Amazon Web Services provides cloud computing services to businesses worldwide."),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("detect", output)
}

func TestComprehend_DetectPiiEntities(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	output, err := client.DetectPiiEntities(ctx, &comprehend.DetectPiiEntitiesInput{
		LanguageCode: types.LanguageCodeEn,
		Text:         aws.String("My email is john.doe@example.com and my phone number is 555-123-4567."),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("detect", output)
}

func TestComprehend_DetectSyntax(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	output, err := client.DetectSyntax(ctx, &comprehend.DetectSyntaxInput{
		LanguageCode: types.SyntaxLanguageCodeEn,
		Text:         aws.String("The quick brown fox jumps over the lazy dog."),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("detect", output)
}

func TestComprehend_ContainsPiiEntities(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	output, err := client.ContainsPiiEntities(ctx, &comprehend.ContainsPiiEntitiesInput{
		LanguageCode: types.LanguageCodeEn,
		Text:         aws.String("Contact me at user@example.com or call 555-123-4567."),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("contains", output)
}

func TestComprehend_BatchDetectSentiment(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	output, err := client.BatchDetectSentiment(ctx, &comprehend.BatchDetectSentimentInput{
		LanguageCode: types.LanguageCodeEn,
		TextList: []string{
			"I love this!",
			"This is terrible.",
			"The weather is nice today.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("batch", output)
}

func TestComprehend_BatchDetectDominantLanguage(t *testing.T) {
	client := newComprehendClient(t)
	ctx := t.Context()

	output, err := client.BatchDetectDominantLanguage(ctx, &comprehend.BatchDetectDominantLanguageInput{
		TextList: []string{
			"Hello, how are you?",
			"Bonjour, comment allez-vous?",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("batch", output)
}
