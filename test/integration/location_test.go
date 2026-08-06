//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/location"
	"github.com/aws/aws-sdk-go-v2/service/location/types"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/sivchari/golden"
)

func newLocationClient(t *testing.T) *location.Client {
	t.Helper()

	return location.NewFromConfig(awsConfig(t), func(o *location.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
		// Disable host prefix (e.g., "cp.maps.") to route requests to localhost.
		o.APIOptions = append(o.APIOptions, func(stack *smithymiddleware.Stack) error {
			return stack.Serialize.Add(smithymiddleware.SerializeMiddlewareFunc(
				"DisableHostPrefix",
				func(ctx context.Context, in smithymiddleware.SerializeInput, next smithymiddleware.SerializeHandler) (smithymiddleware.SerializeOutput, smithymiddleware.Metadata, error) {
					ctx = smithyhttp.DisableEndpointHostPrefix(ctx, true)
					return next.HandleSerialize(ctx, in)
				},
			), smithymiddleware.Before)
		})
	})
}

func TestLocation_CreateAndDeleteMap(t *testing.T) {
	client := newLocationClient(t)
	ctx := t.Context()

	mapName := "test-map"

	createOutput, err := client.CreateMap(ctx, &location.CreateMapInput{
		MapName: aws.String(mapName),
		Configuration: &types.MapConfiguration{
			Style: aws.String("VectorHereExplore"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteMap(context.Background(), &location.DeleteMapInput{
			MapName: aws.String(mapName),
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("MapArn", "CreateTime", "UpdateTime", "ResultMetadata"))
	g.Assert(t.Name()+"/CreateMap", createOutput)

	// Describe map.
	descOutput, err := client.DescribeMap(ctx, &location.DescribeMapInput{
		MapName: aws.String(mapName),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribeMap", descOutput)

	// Delete map.
	_, err = client.DeleteMap(ctx, &location.DeleteMapInput{
		MapName: aws.String(mapName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify map is deleted.
	_, err = client.DescribeMap(ctx, &location.DescribeMapInput{
		MapName: aws.String(mapName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestLocation_ListMaps(t *testing.T) {
	client := newLocationClient(t)
	ctx := t.Context()

	mapName := "test-list-map"

	_, err := client.CreateMap(ctx, &location.CreateMapInput{
		MapName: aws.String(mapName),
		Configuration: &types.MapConfiguration{
			Style: aws.String("VectorHereExplore"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteMap(context.Background(), &location.DeleteMapInput{
			MapName: aws.String(mapName),
		})
	})

	listOutput, err := client.ListMaps(ctx, &location.ListMapsInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(listOutput.Entries) < 1 {
		t.Errorf("expected at least 1 map, got %d", len(listOutput.Entries))
	}
}

func TestLocation_CreateAndDeletePlaceIndex(t *testing.T) {
	client := newLocationClient(t)
	ctx := t.Context()

	indexName := "test-place-index"

	createOutput, err := client.CreatePlaceIndex(ctx, &location.CreatePlaceIndexInput{
		IndexName:  aws.String(indexName),
		DataSource: aws.String("Esri"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeletePlaceIndex(context.Background(), &location.DeletePlaceIndexInput{
			IndexName: aws.String(indexName),
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("IndexArn", "CreateTime", "UpdateTime", "ResultMetadata"))
	g.Assert(t.Name()+"/CreatePlaceIndex", createOutput)

	// Describe place index.
	descOutput, err := client.DescribePlaceIndex(ctx, &location.DescribePlaceIndexInput{
		IndexName: aws.String(indexName),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribePlaceIndex", descOutput)

	// Delete place index.
	_, err = client.DeletePlaceIndex(ctx, &location.DeletePlaceIndexInput{
		IndexName: aws.String(indexName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify place index is deleted.
	_, err = client.DescribePlaceIndex(ctx, &location.DescribePlaceIndexInput{
		IndexName: aws.String(indexName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestLocation_CreateAndDeleteRouteCalculator(t *testing.T) {
	client := newLocationClient(t)
	ctx := t.Context()

	calcName := "test-route-calculator"

	createOutput, err := client.CreateRouteCalculator(ctx, &location.CreateRouteCalculatorInput{
		CalculatorName: aws.String(calcName),
		DataSource:     aws.String("Esri"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRouteCalculator(context.Background(), &location.DeleteRouteCalculatorInput{
			CalculatorName: aws.String(calcName),
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("CalculatorArn", "CreateTime", "UpdateTime", "ResultMetadata"))
	g.Assert(t.Name()+"/CreateRouteCalculator", createOutput)

	// Describe route calculator.
	descOutput, err := client.DescribeRouteCalculator(ctx, &location.DescribeRouteCalculatorInput{
		CalculatorName: aws.String(calcName),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribeRouteCalculator", descOutput)

	// Delete route calculator.
	_, err = client.DeleteRouteCalculator(ctx, &location.DeleteRouteCalculatorInput{
		CalculatorName: aws.String(calcName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify route calculator is deleted.
	_, err = client.DescribeRouteCalculator(ctx, &location.DescribeRouteCalculatorInput{
		CalculatorName: aws.String(calcName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestLocation_CreateAndDeleteGeofenceCollection(t *testing.T) {
	client := newLocationClient(t)
	ctx := t.Context()

	collName := "test-geofence-collection"

	createOutput, err := client.CreateGeofenceCollection(ctx, &location.CreateGeofenceCollectionInput{
		CollectionName: aws.String(collName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteGeofenceCollection(context.Background(), &location.DeleteGeofenceCollectionInput{
			CollectionName: aws.String(collName),
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("CollectionArn", "CreateTime", "UpdateTime", "ResultMetadata"))
	g.Assert(t.Name()+"/CreateGeofenceCollection", createOutput)

	// Describe geofence collection.
	descOutput, err := client.DescribeGeofenceCollection(ctx, &location.DescribeGeofenceCollectionInput{
		CollectionName: aws.String(collName),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribeGeofenceCollection", descOutput)

	// Delete geofence collection.
	_, err = client.DeleteGeofenceCollection(ctx, &location.DeleteGeofenceCollectionInput{
		CollectionName: aws.String(collName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify geofence collection is deleted.
	_, err = client.DescribeGeofenceCollection(ctx, &location.DescribeGeofenceCollectionInput{
		CollectionName: aws.String(collName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestLocation_CreateAndDeleteTracker(t *testing.T) {
	client := newLocationClient(t)
	ctx := t.Context()

	trackerName := "test-tracker"

	createOutput, err := client.CreateTracker(ctx, &location.CreateTrackerInput{
		TrackerName: aws.String(trackerName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTracker(context.Background(), &location.DeleteTrackerInput{
			TrackerName: aws.String(trackerName),
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("TrackerArn", "CreateTime", "UpdateTime", "ResultMetadata"))
	g.Assert(t.Name()+"/CreateTracker", createOutput)

	// Describe tracker.
	descOutput, err := client.DescribeTracker(ctx, &location.DescribeTrackerInput{
		TrackerName: aws.String(trackerName),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribeTracker", descOutput)

	// Delete tracker.
	_, err = client.DeleteTracker(ctx, &location.DeleteTrackerInput{
		TrackerName: aws.String(trackerName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify tracker is deleted.
	_, err = client.DescribeTracker(ctx, &location.DescribeTrackerInput{
		TrackerName: aws.String(trackerName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestLocation_MapNotFound(t *testing.T) {
	client := newLocationClient(t)
	ctx := t.Context()

	_, err := client.DescribeMap(ctx, &location.DescribeMapInput{
		MapName: aws.String("non-existent-map"),
	})
	if err == nil {
		t.Error("expected error")
	}

	_, err = client.DeleteMap(ctx, &location.DeleteMapInput{
		MapName: aws.String("non-existent-map"),
	})
	if err == nil {
		t.Error("expected error")
	}
}
