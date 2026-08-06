//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/sivchari/golden"
)

func newRekognitionClient(t *testing.T) *rekognition.Client {
	t.Helper()

	return rekognition.NewFromConfig(awsConfig(t), func(o *rekognition.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestRekognition_CreateAndDeleteCollection(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	// Create collection
	collectionId := "test-collection"

	createOutput, err := client.CreateCollection(ctx, &rekognition.CreateCollectionInput{
		CollectionId: aws.String(collectionId),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "CollectionArn"),
	)
	g.Assert(t.Name()+"_create", createOutput)

	// Delete collection
	deleteOutput, err := client.DeleteCollection(ctx, &rekognition.DeleteCollectionInput{
		CollectionId: aws.String(collectionId),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g2.Assert(t.Name()+"_delete", deleteOutput)
}

func TestRekognition_ListCollections(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	// Create a collection first
	_, err := client.CreateCollection(ctx, &rekognition.CreateCollectionInput{
		CollectionId: aws.String("test-list-collection"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCollection(ctx, &rekognition.DeleteCollectionInput{
			CollectionId: aws.String("test-list-collection"),
		})
	})

	// List collections
	output, err := client.ListCollections(ctx, &rekognition.ListCollectionsInput{})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "CollectionIds", "FaceModelVersions"),
	)
	g.Assert(t.Name(), output)
}

func TestRekognition_DescribeCollection(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	collectionId := "test-describe-collection"

	// Create collection
	_, err := client.CreateCollection(ctx, &rekognition.CreateCollectionInput{
		CollectionId: aws.String(collectionId),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCollection(ctx, &rekognition.DeleteCollectionInput{
			CollectionId: aws.String(collectionId),
		})
	})

	// Describe collection
	output, err := client.DescribeCollection(ctx, &rekognition.DescribeCollectionInput{
		CollectionId: aws.String(collectionId),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "CollectionARN", "CreationTimestamp"),
	)
	g.Assert(t.Name(), output)
}

func TestRekognition_DetectFaces(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	// Detect faces with a minimal image (mock returns predefined data)
	output, err := client.DetectFaces(ctx, &rekognition.DetectFacesInput{
		Image: &types.Image{
			Bytes: []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header bytes (mock)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert(t.Name(), output)
}

func TestRekognition_DetectLabels(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	// Detect labels with a minimal image
	output, err := client.DetectLabels(ctx, &rekognition.DetectLabelsInput{
		Image: &types.Image{
			Bytes: []byte{0x89, 0x50, 0x4E, 0x47},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert(t.Name(), output)
}

func TestRekognition_DetectText(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	// Detect text with a minimal image
	output, err := client.DetectText(ctx, &rekognition.DetectTextInput{
		Image: &types.Image{
			Bytes: []byte{0x89, 0x50, 0x4E, 0x47},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert(t.Name(), output)
}

func TestRekognition_RecognizeCelebrities(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	// Recognize celebrities with a minimal image
	output, err := client.RecognizeCelebrities(ctx, &rekognition.RecognizeCelebritiesInput{
		Image: &types.Image{
			Bytes: []byte{0x89, 0x50, 0x4E, 0x47},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert(t.Name(), output)
}

func TestRekognition_DetectModerationLabels(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	// Detect moderation labels with a minimal image
	output, err := client.DetectModerationLabels(ctx, &rekognition.DetectModerationLabelsInput{
		Image: &types.Image{
			Bytes: []byte{0x89, 0x50, 0x4E, 0x47},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert(t.Name(), output)
}

func TestRekognition_IndexAndSearchFaces(t *testing.T) {
	client := newRekognitionClient(t)
	ctx := t.Context()

	collectionId := "test-face-collection"

	// Create collection
	_, err := client.CreateCollection(ctx, &rekognition.CreateCollectionInput{
		CollectionId: aws.String(collectionId),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCollection(ctx, &rekognition.DeleteCollectionInput{
			CollectionId: aws.String(collectionId),
		})
	})

	// Index a face
	indexOutput, err := client.IndexFaces(ctx, &rekognition.IndexFacesInput{
		CollectionId:    aws.String(collectionId),
		ExternalImageId: aws.String("test-person-1"),
		Image: &types.Image{
			Bytes: []byte{0x89, 0x50, 0x4E, 0x47},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "FaceRecords", "FaceModelVersion"),
	)
	g.Assert(t.Name()+"_index", indexOutput)

	// List faces
	listOutput, err := client.ListFaces(ctx, &rekognition.ListFacesInput{
		CollectionId: aws.String(collectionId),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Faces", "FaceModelVersion"),
	)
	g2.Assert(t.Name()+"_list", listOutput)
}
