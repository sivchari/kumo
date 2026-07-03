//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/sivchari/golden"
)

func newRoute53Client(t *testing.T) *route53.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return route53.NewFromConfig(cfg, func(o *route53.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestRoute53_CreateHostedZone(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	result, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("example.com"),
		CallerReference: aws.String("test-create-hosted-zone"),
		HostedZoneConfig: &types.HostedZoneConfig{
			Comment:     aws.String("Test hosted zone"),
			PrivateZone: false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, err := client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
			Id: result.HostedZone.Id,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"SubmittedAt",
		"NameServers",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), result)
}

func TestRoute53_GetHostedZone(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	// Create hosted zone first.
	createResult, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("get-test.example.com"),
		CallerReference: aws.String("test-get-hosted-zone"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, err := client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
			Id: createResult.HostedZone.Id,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	// Get hosted zone.
	getResult, err := client.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: createResult.HostedZone.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"ResourceRecordSetCount",
		"NameServers",
		"ResultMetadata",
	)).Assert(t.Name(), getResult)
}

func TestRoute53_ListHostedZones(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	// Create hosted zone first.
	createResult, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("list-test.example.com"),
		CallerReference: aws.String("test-list-hosted-zones"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, err := client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
			Id: createResult.HostedZone.Id,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	// List hosted zones.
	listResult, err := client.ListHostedZones(ctx, &route53.ListHostedZonesInput{})
	if err != nil {
		t.Fatal(err)
	}

	// Find our hosted zone.
	found := false
	for _, zone := range listResult.HostedZones {
		if *zone.Id == *createResult.HostedZone.Id {
			found = true
			break
		}
	}
	if !found {
		t.Error("Hosted zone should be in list")
	}
}

func TestRoute53_DeleteHostedZone(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	// Create hosted zone first.
	createResult, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("delete-test.example.com"),
		CallerReference: aws.String("test-delete-hosted-zone"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete hosted zone.
	deleteResult, err := client.DeleteHostedZone(ctx, &route53.DeleteHostedZoneInput{
		Id: createResult.HostedZone.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"SubmittedAt",
		"ResultMetadata",
	)).Assert(t.Name(), deleteResult)

	// Verify it's deleted.
	_, err = client.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: createResult.HostedZone.Id,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestRoute53_ChangeResourceRecordSets(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	// Create hosted zone first.
	createResult, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("records-test.example.com"),
		CallerReference: aws.String("test-change-record-sets"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		// Delete records first.
		_, _ = client.ChangeResourceRecordSets(context.Background(), &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: createResult.HostedZone.Id,
			ChangeBatch: &types.ChangeBatch{
				Changes: []types.Change{
					{
						Action: types.ChangeActionDelete,
						ResourceRecordSet: &types.ResourceRecordSet{
							Name: aws.String("www.records-test.example.com."),
							Type: types.RRTypeA,
							TTL:  aws.Int64(300),
							ResourceRecords: []types.ResourceRecord{
								{Value: aws.String("192.0.2.1")},
							},
						},
					},
				},
			},
		})
		_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
			Id: createResult.HostedZone.Id,
		})
	})

	// Create record set.
	changeResult, err := client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: createResult.HostedZone.Id,
		ChangeBatch: &types.ChangeBatch{
			Comment: aws.String("Adding A record"),
			Changes: []types.Change{
				{
					Action: types.ChangeActionCreate,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String("www.records-test.example.com."),
						Type: types.RRTypeA,
						TTL:  aws.Int64(300),
						ResourceRecords: []types.ResourceRecord{
							{Value: aws.String("192.0.2.1")},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"SubmittedAt",
		"ResultMetadata",
	)).Assert(t.Name(), changeResult)
}

func TestRoute53_ListResourceRecordSets(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	// Create hosted zone first.
	createResult, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("list-records-test.example.com"),
		CallerReference: aws.String("test-list-record-sets"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		// Delete records first.
		_, _ = client.ChangeResourceRecordSets(context.Background(), &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: createResult.HostedZone.Id,
			ChangeBatch: &types.ChangeBatch{
				Changes: []types.Change{
					{
						Action: types.ChangeActionDelete,
						ResourceRecordSet: &types.ResourceRecordSet{
							Name: aws.String("api.list-records-test.example.com."),
							Type: types.RRTypeCname,
							TTL:  aws.Int64(300),
							ResourceRecords: []types.ResourceRecord{
								{Value: aws.String("app.example.com")},
							},
						},
					},
				},
			},
		})
		_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
			Id: createResult.HostedZone.Id,
		})
	})

	// Create a record set.
	_, err = client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: createResult.HostedZone.Id,
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionCreate,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String("api.list-records-test.example.com."),
						Type: types.RRTypeCname,
						TTL:  aws.Int64(300),
						ResourceRecords: []types.ResourceRecord{
							{Value: aws.String("app.example.com")},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List record sets.
	listResult, err := client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: createResult.HostedZone.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Find our record.
	found := false
	for _, record := range listResult.ResourceRecordSets {
		if *record.Name == "api.list-records-test.example.com." && record.Type == types.RRTypeCname {
			found = true
			break
		}
	}
	if !found {
		t.Error("Record set should be in list")
	}
}

func TestRoute53_UpsertResourceRecordSet(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	// Create hosted zone first.
	createResult, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("upsert-test.example.com"),
		CallerReference: aws.String("test-upsert-record-set"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		// Delete records first.
		_, _ = client.ChangeResourceRecordSets(context.Background(), &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: createResult.HostedZone.Id,
			ChangeBatch: &types.ChangeBatch{
				Changes: []types.Change{
					{
						Action: types.ChangeActionDelete,
						ResourceRecordSet: &types.ResourceRecordSet{
							Name: aws.String("test.upsert-test.example.com."),
							Type: types.RRTypeA,
							TTL:  aws.Int64(600),
							ResourceRecords: []types.ResourceRecord{
								{Value: aws.String("192.0.2.2")},
							},
						},
					},
				},
			},
		})
		_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
			Id: createResult.HostedZone.Id,
		})
	})

	// Upsert (create) record set.
	_, err = client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: createResult.HostedZone.Id,
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String("test.upsert-test.example.com."),
						Type: types.RRTypeA,
						TTL:  aws.Int64(300),
						ResourceRecords: []types.ResourceRecord{
							{Value: aws.String("192.0.2.1")},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Upsert (update) record set.
	_, err = client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: createResult.HostedZone.Id,
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String("test.upsert-test.example.com."),
						Type: types.RRTypeA,
						TTL:  aws.Int64(600),
						ResourceRecords: []types.ResourceRecord{
							{Value: aws.String("192.0.2.2")},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify update.
	listResult, err := client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: createResult.HostedZone.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, record := range listResult.ResourceRecordSets {
		if *record.Name == "test.upsert-test.example.com." && record.Type == types.RRTypeA {
			found = true
			if *record.TTL != 600 {
				t.Errorf("expected TTL 600, got %d", *record.TTL)
			}
			if *record.ResourceRecords[0].Value != "192.0.2.2" {
				t.Errorf("expected value 192.0.2.2, got %s", *record.ResourceRecords[0].Value)
			}
			break
		}
	}
	if !found {
		t.Error("Updated record set should be in list")
	}
}

func TestRoute53_ListHostedZones_Pagination(t *testing.T) {
	// Not parallel: pagination assertions require a stable view of the
	// hosted zones list, which other parallel tests mutate.
	client := newRoute53Client(t)
	ctx := t.Context()

	// Create multiple hosted zones for pagination test.
	var createdZones []*route53.CreateHostedZoneOutput
	for i := 0; i < 3; i++ {
		result, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
			Name:            aws.String(fmt.Sprintf("pagination-test-%d.example.com", i)),
			CallerReference: aws.String(fmt.Sprintf("test-pagination-%d-%d", i, time.Now().UnixNano())),
		})
		if err != nil {
			t.Fatal(err)
		}
		createdZones = append(createdZones, result)
	}

	t.Cleanup(func() {
		for _, zone := range createdZones {
			_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
				Id: zone.HostedZone.Id,
			})
		}
	})

	// Test with MaxItems=1 to force pagination.
	firstPage, err := client.ListHostedZones(ctx, &route53.ListHostedZonesInput{
		MaxItems: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if *firstPage.MaxItems != 1 {
		t.Errorf("expected MaxItems 1, got %d", *firstPage.MaxItems)
	}
	if len(firstPage.HostedZones) != 1 {
		t.Errorf("expected 1 hosted zone, got %d", len(firstPage.HostedZones))
	}

	// If there are more results, IsTruncated should be true.
	if firstPage.IsTruncated {
		if firstPage.NextMarker == nil || *firstPage.NextMarker == "" {
			t.Error("expected NextMarker to be set when IsTruncated is true")
		}

		// Get the next page using the marker.
		secondPage, err := client.ListHostedZones(ctx, &route53.ListHostedZonesInput{
			MaxItems: aws.Int32(1),
			Marker:   firstPage.NextMarker,
		})
		if err != nil {
			t.Fatal(err)
		}
		if *secondPage.Marker != *firstPage.NextMarker {
			t.Errorf("expected Marker %s, got %s", *firstPage.NextMarker, *secondPage.Marker)
		}
		if len(secondPage.HostedZones) != 1 {
			t.Errorf("expected 1 hosted zone, got %d", len(secondPage.HostedZones))
		}

		// The second page should have a different zone.
		if *firstPage.HostedZones[0].Id == *secondPage.HostedZones[0].Id {
			t.Error("expected different hosted zone IDs on different pages")
		}
	}
}

func TestRoute53_GetChange(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	createResult, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("getchange-test.example.com"),
		CallerReference: aws.String("test-get-change"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
			Id: createResult.HostedZone.Id,
		})
	})

	getResult, err := client.GetChange(ctx, &route53.GetChangeInput{
		Id: createResult.ChangeInfo.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	if getResult.ChangeInfo == nil || getResult.ChangeInfo.Id == nil {
		t.Fatalf("expected ChangeInfo with Id, got %+v", getResult.ChangeInfo)
	}

	if *getResult.ChangeInfo.Id != *createResult.ChangeInfo.Id {
		t.Errorf("expected change Id %s, got %s", *createResult.ChangeInfo.Id, *getResult.ChangeInfo.Id)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id", "SubmittedAt", "ResultMetadata",
	)).Assert(t.Name(), getResult)
}

func TestRoute53_GetChange_NotFound(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	_, err := client.GetChange(ctx, &route53.GetChangeInput{
		Id: aws.String("nonexistent-change-id"),
	})
	if err == nil {
		t.Fatal("expected error for nonexistent change, got nil")
	}

	var nfe *types.NoSuchChange
	if !errors.As(err, &nfe) {
		t.Fatalf("expected NoSuchChange error, got %T: %v", err, err)
	}
}

func TestRoute53_TagOperations(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	// Create a hosted zone for tagging.
	createResult, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("tag-test.example.com"),
		CallerReference: aws.String(fmt.Sprintf("test-tag-ops-%d", time.Now().UnixNano())),
	})
	if err != nil {
		t.Fatal(err)
	}

	hostedZoneID := createResult.HostedZone.Id
	// The SDK does not sanitize ResourceId for tag operations, so we must
	// pass the bare zone ID (without the "/hostedzone/" prefix) to avoid
	// percent-encoded slashes in the URL path.
	bareZoneID := aws.String(strings.TrimPrefix(*hostedZoneID, "/hostedzone/"))

	t.Cleanup(func() {
		_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
			Id: hostedZoneID,
		})
	})

	// Add tags.
	_, err = client.ChangeTagsForResource(ctx, &route53.ChangeTagsForResourceInput{
		ResourceType: types.TagResourceTypeHostedzone,
		ResourceId:   bareZoneID,
		AddTags: []types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("test")},
			{Key: aws.String("Project"), Value: aws.String("kumo")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List tags after adding.
	listResult, err := client.ListTagsForResource(ctx, &route53.ListTagsForResourceInput{
		ResourceType: types.TagResourceTypeHostedzone,
		ResourceId:   bareZoneID,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ResourceId", "ResultMetadata",
	)).Assert(t.Name()+"_after_add", listResult)

	// Remove one tag.
	_, err = client.ChangeTagsForResource(ctx, &route53.ChangeTagsForResourceInput{
		ResourceType: types.TagResourceTypeHostedzone,
		ResourceId:   bareZoneID,
		RemoveTagKeys: []string{
			"Project",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List tags after removing.
	listResult, err = client.ListTagsForResource(ctx, &route53.ListTagsForResourceInput{
		ResourceType: types.TagResourceTypeHostedzone,
		ResourceId:   bareZoneID,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ResourceId", "ResultMetadata",
	)).Assert(t.Name()+"_after_remove", listResult)
}

func TestRoute53_ListHostedZonesByName(t *testing.T) {
	client := newRoute53Client(t)
	ctx := t.Context()

	// Create three zones with names that span the alphabet so we can verify
	// the ascending-order start filter.
	for i, name := range []string{"alpha.example.com.", "kumo-zone.example.com.", "zeta.example.com."} {
		ref := fmt.Sprintf("lhzbn-test-%d-%d", time.Now().UnixNano(), i)

		zoneRes, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
			Name:            aws.String(name),
			CallerReference: aws.String(ref),
		})
		if err != nil {
			t.Fatalf("CreateHostedZone(%s): %v", name, err)
		}

		id := *zoneRes.HostedZone.Id
		t.Cleanup(func() {
			_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{
				Id: aws.String(id),
			})
		})
	}

	// No filter: returns all zones, sorted ascending by name.
	allRes, err := client.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{})
	if err != nil {
		t.Fatalf("ListHostedZonesByName (no filter): %v", err)
	}

	if got := len(allRes.HostedZones); got < 3 {
		t.Fatalf("ListHostedZonesByName returned %d zones, want at least 3", got)
	}

	for i := 1; i < len(allRes.HostedZones); i++ {
		if *allRes.HostedZones[i-1].Name > *allRes.HostedZones[i].Name {
			t.Errorf("zones not sorted ascending: %q > %q", *allRes.HostedZones[i-1].Name, *allRes.HostedZones[i].Name)
		}
	}

	// Filter by exact name: the first zone in the response is the requested
	// zone (the index-0 contract used by data source-style lookups).
	exactRes, err := client.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String("kumo-zone.example.com"),
	})
	if err != nil {
		t.Fatalf("ListHostedZonesByName (dnsname): %v", err)
	}

	if got := len(exactRes.HostedZones); got == 0 {
		t.Fatal("ListHostedZonesByName(dnsname=kumo-zone.example.com) returned 0 zones")
	}

	if got := *exactRes.HostedZones[0].Name; got != "kumo-zone.example.com." {
		t.Errorf("first zone for exact-name query = %q, want kumo-zone.example.com.", got)
	}
}
