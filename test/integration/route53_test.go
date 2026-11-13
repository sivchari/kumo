//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

func newRoute53Client(t *testing.T) *route53.Client {
	t.Helper()

	return route53.NewFromConfig(awsConfig(t), func(o *route53.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
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

// zonePlaceholder stands in for a per-run hosted zone name in golden files.
const zonePlaceholder = "{zone}"

// route53GoldenIgnores hides the per-run identifiers of a hosted zone.
var route53GoldenIgnores = golden.WithIgnoreFields("Id", "CallerReference", "NameServers", "ResultMetadata")

// createRecordTestZone creates a hosted zone whose name is unique to this
// run, so a rerun against a persisted server never meets a leftover zone, and
// returns its ID and name. On cleanup it deletes the record sets the test
// left behind and the zone.
func createRecordTestZone(t *testing.T, client *route53.Client, prefix string) (zoneID, zoneName string) {
	t.Helper()

	label := uniqueName(prefix)
	zoneName = label + ".example.com"

	created, err := client.CreateHostedZone(t.Context(), &route53.CreateHostedZoneInput{
		Name:            aws.String(zoneName),
		CallerReference: aws.String(label),
	})
	if err != nil {
		t.Fatal(err)
	}

	zoneID = aws.ToString(created.HostedZone.Id)

	t.Cleanup(func() {
		ctx := context.Background()

		listed, err := client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)})
		if err == nil {
			for i := range listed.ResourceRecordSets {
				_ = changeRecordSets(ctx, client, zoneID, types.Change{
					Action:            types.ChangeActionDelete,
					ResourceRecordSet: &listed.ResourceRecordSets[i],
				})
			}
		}

		_, _ = client.DeleteHostedZone(ctx, &route53.DeleteHostedZoneInput{Id: aws.String(zoneID)})
	})

	return zoneID, zoneName
}

// changeRecordSets sends the changes to the zone as one change batch.
func changeRecordSets(ctx context.Context, client *route53.Client, zoneID string, changes ...types.Change) error {
	_, err := client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch:  &types.ChangeBatch{Changes: changes},
	})

	return err
}

// aRecordChange returns a change of an A record set with the given TTL and
// values.
func aRecordChange(action types.ChangeAction, name string, ttl int64, values ...string) types.Change {
	records := make([]types.ResourceRecord, 0, len(values))
	for _, value := range values {
		records = append(records, types.ResourceRecord{Value: aws.String(value)})
	}

	return types.Change{
		Action: action,
		ResourceRecordSet: &types.ResourceRecordSet{
			Name:            aws.String(name),
			Type:            types.RRTypeA,
			TTL:             aws.Int64(ttl),
			ResourceRecords: records,
		},
	}
}

// listRecordSets lists the record sets of the zone.
func listRecordSets(t *testing.T, client *route53.Client, zoneID string) *route53.ListResourceRecordSetsOutput {
	t.Helper()

	listed, err := client.ListResourceRecordSets(t.Context(), &route53.ListResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)})
	if err != nil {
		t.Fatal(err)
	}

	return listed
}

// assertZoneGolden golden-asserts v with the per-run zone name replaced by
// zonePlaceholder, so the golden file still pins record names.
func assertZoneGolden(t *testing.T, name, zoneName string, v any) {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, route53GoldenIgnores).Assert(name, strings.ReplaceAll(string(data), zoneName, zonePlaceholder))
}

func TestRoute53_RecordNameWithoutTrailingDot(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()
	zoneID, zoneName := createRecordTestZone(t, client, "fqdn-test")

	// Create without the trailing dot; the record set is listed fully
	// qualified.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionCreate, "www."+zoneName, 300, "192.0.2.1")); err != nil {
		t.Fatal(err)
	}

	assertZoneGolden(t, t.Name()+"_create", zoneName, listRecordSets(t, client, zoneID))

	// Creating the same name with the trailing dot is a duplicate.
	var apiErr smithy.APIError
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionCreate, "www."+zoneName+".", 300, "192.0.2.1")); !errors.As(err, &apiErr) {
		t.Fatalf("expected an API error for duplicate record, got %T: %v", err, err)
	}

	// Delete with the trailing dot.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionDelete, "www."+zoneName+".", 300, "192.0.2.1")); err != nil {
		t.Fatal(err)
	}

	assertZoneGolden(t, t.Name()+"_delete", zoneName, listRecordSets(t, client, zoneID))
}

func TestRoute53_RecordNameIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()
	zoneID, zoneName := createRecordTestZone(t, client, "case-test")
	upperZoneName := strings.ToUpper(zoneName)

	// Create in upper case; the record set is listed in lower case.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionCreate, "WWW."+upperZoneName, 300, "192.0.2.1")); err != nil {
		t.Fatal(err)
	}

	assertZoneGolden(t, t.Name()+"_create", zoneName, listRecordSets(t, client, zoneID))

	// Creating the same name in lower case is a duplicate.
	var apiErr smithy.APIError
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionCreate, "www."+zoneName+".", 300, "192.0.2.1")); !errors.As(err, &apiErr) {
		t.Fatalf("expected an API error for duplicate record, got %T: %v", err, err)
	}

	// UPSERT in mixed case updates the same record set.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionUpsert, "Www."+zoneName, 600, "192.0.2.2")); err != nil {
		t.Fatal(err)
	}

	assertZoneGolden(t, t.Name()+"_upsert", zoneName, listRecordSets(t, client, zoneID))

	// DELETE in yet another case removes it.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionDelete, "wWw."+upperZoneName+".", 600, "192.0.2.2")); err != nil {
		t.Fatal(err)
	}

	assertZoneGolden(t, t.Name()+"_delete", zoneName, listRecordSets(t, client, zoneID))
}

func TestRoute53_ChangeResourceRecordSets_EmptyName(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()
	zoneID, zoneName := createRecordTestZone(t, client, "empty-name-test")

	// The SDK rejects a nil Name before sending the request, so send an
	// empty one.
	err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionCreate, "", 300, "192.0.2.1"))
	assertAPIErrorCode(t, err, "InvalidInput")

	// Nothing was stored, not even under the root name.
	assertZoneGolden(t, t.Name()+"_list", zoneName, listRecordSets(t, client, zoneID))
}

func TestRoute53_ChangeBatchIsAtomic(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()
	zoneID, zoneName := createRecordTestZone(t, client, "atomic-test")
	www := "www." + zoneName + "."
	api := "api." + zoneName + "."

	if err := changeRecordSets(ctx, client, zoneID,
		aRecordChange(types.ChangeActionCreate, www, 300, "192.0.2.1"),
		aRecordChange(types.ChangeActionCreate, api, 300, "192.0.2.10"),
	); err != nil {
		t.Fatal(err)
	}

	// Each batch fails on its last change, a duplicate CREATE, after a DELETE
	// or UPSERT that would succeed on its own. Neither may change the zone.
	duplicate := aRecordChange(types.ChangeActionCreate, api, 300, "192.0.2.10")

	for _, first := range []types.Change{
		aRecordChange(types.ChangeActionDelete, www, 300, "192.0.2.1"),
		aRecordChange(types.ChangeActionUpsert, www, 600, "192.0.2.2"),
	} {
		var apiErr smithy.APIError
		if err := changeRecordSets(ctx, client, zoneID, first, duplicate); !errors.As(err, &apiErr) {
			t.Fatalf("%s then duplicate CREATE: expected an API error, got %T: %v", first.Action, err, err)
		}
	}

	assertZoneGolden(t, t.Name()+"_records", zoneName, listRecordSets(t, client, zoneID))

	zone, err := client.GetHostedZone(ctx, &route53.GetHostedZoneInput{Id: aws.String(zoneID)})
	if err != nil {
		t.Fatal(err)
	}

	assertZoneGolden(t, t.Name()+"_zone", zoneName, zone)
}

func TestRoute53_WeightedRecordSets(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()
	zoneID, zoneName := createRecordTestZone(t, client, "weighted-test")
	www := "www." + zoneName + "."

	weighted := func(action types.ChangeAction, setIdentifier, value string, weight int64) types.Change {
		change := aRecordChange(action, www, 60, value)
		change.ResourceRecordSet.SetIdentifier = aws.String(setIdentifier)
		change.ResourceRecordSet.Weight = aws.Int64(weight)

		return change
	}

	// Record sets that share a name and type but not a SetIdentifier are
	// distinct.
	if err := changeRecordSets(ctx, client, zoneID,
		weighted(types.ChangeActionCreate, "blue", "192.0.2.1", 10),
		weighted(types.ChangeActionCreate, "green", "192.0.2.2", 90),
	); err != nil {
		t.Fatal(err)
	}

	assertZoneGolden(t, t.Name()+"_create", zoneName, listRecordSets(t, client, zoneID))

	// UPSERT and DELETE act only on the record set with their SetIdentifier.
	if err := changeRecordSets(ctx, client, zoneID,
		weighted(types.ChangeActionUpsert, "green", "192.0.2.3", 80),
		weighted(types.ChangeActionDelete, "blue", "192.0.2.1", 10),
	); err != nil {
		t.Fatal(err)
	}

	assertZoneGolden(t, t.Name()+"_change", zoneName, listRecordSets(t, client, zoneID))
}

func TestRoute53_RoutingPolicyConflict(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()
	zoneID, zoneName := createRecordTestZone(t, client, "routing-test")
	www := "www." + zoneName + "."

	identified := func(action types.ChangeAction, setIdentifier, value string, policy func(*types.ResourceRecordSet)) types.Change {
		change := aRecordChange(action, www, 60, value)
		change.ResourceRecordSet.SetIdentifier = aws.String(setIdentifier)
		policy(change.ResourceRecordSet)

		return change
	}
	weighted := func(setIdentifier, value string, weight int64) types.Change {
		return identified(types.ChangeActionCreate, setIdentifier, value, func(rrset *types.ResourceRecordSet) {
			rrset.Weight = aws.Int64(weight)
		})
	}
	latency := identified(types.ChangeActionCreate, "east", "192.0.2.3", func(rrset *types.ResourceRecordSet) {
		rrset.Region = types.ResourceRecordSetRegionUsEast1
	})
	failover := identified(types.ChangeActionCreate, "primary", "192.0.2.4", func(rrset *types.ResourceRecordSet) {
		rrset.Failover = types.ResourceRecordSetFailoverPrimary
	})

	// A simple record set can't share its name and type with a latency or a
	// weighted record set.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionCreate, www, 60, "192.0.2.1")); err != nil {
		t.Fatal(err)
	}

	assertAPIErrorCode(t, changeRecordSets(ctx, client, zoneID, latency), "InvalidChangeBatch")
	assertAPIErrorCode(t, changeRecordSets(ctx, client, zoneID, weighted("blue", "192.0.2.1", 10)), "InvalidChangeBatch")
	assertZoneGolden(t, t.Name()+"_simple", zoneName, listRecordSets(t, client, zoneID))

	// The check applies to the state the whole batch leaves, so one batch can
	// replace the simple record set with weighted ones.
	if err := changeRecordSets(ctx, client, zoneID,
		aRecordChange(types.ChangeActionDelete, www, 60, "192.0.2.1"),
		weighted("blue", "192.0.2.1", 10),
		weighted("green", "192.0.2.2", 90),
	); err != nil {
		t.Fatal(err)
	}

	// Neither a simple nor a failover record set can join the weighted ones.
	assertAPIErrorCode(t, changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionUpsert, www, 60, "192.0.2.1")), "InvalidChangeBatch")
	assertAPIErrorCode(t, changeRecordSets(ctx, client, zoneID, failover), "InvalidChangeBatch")
	assertZoneGolden(t, t.Name()+"_weighted", zoneName, listRecordSets(t, client, zoneID))
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
