//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

// zonePlaceholder stands in for a per-run hosted zone name in golden files.
const zonePlaceholder = "{zone}"

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

// assertRecordSetsGolden golden-asserts the record sets of the zone, in
// order, with the per-run zone name replaced by zonePlaceholder, so the golden
// file still pins record names.
func assertRecordSetsGolden(t *testing.T, client *route53.Client, name, zoneID, zoneName string) {
	t.Helper()

	listed, err := client.ListResourceRecordSets(t.Context(), &route53.ListResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)})
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(listed.ResourceRecordSets)
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreOrder(false)).Assert(name, strings.ReplaceAll(string(data), zoneName, zonePlaceholder))
}

// assertDuplicateCreate fails unless err is the API error of a CREATE whose
// record set already exists.
func assertDuplicateCreate(t *testing.T, err error) {
	t.Helper()

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an API error for duplicate record, got %T: %v", err, err)
	}
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

	assertRecordSetsGolden(t, client, t.Name()+"_create", zoneID, zoneName)

	// Creating the same name with the trailing dot is a duplicate.
	assertDuplicateCreate(t, changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionCreate, "www."+zoneName+".", 300, "192.0.2.1")))

	// Delete with the trailing dot.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionDelete, "www."+zoneName+".", 300, "192.0.2.1")); err != nil {
		t.Fatal(err)
	}

	assertRecordSetsGolden(t, client, t.Name()+"_delete", zoneID, zoneName)
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

	assertRecordSetsGolden(t, client, t.Name()+"_create", zoneID, zoneName)

	// Creating the same name in lower case is a duplicate.
	assertDuplicateCreate(t, changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionCreate, "www."+zoneName+".", 300, "192.0.2.1")))

	// UPSERT in mixed case updates the same record set.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionUpsert, "Www."+zoneName, 600, "192.0.2.2")); err != nil {
		t.Fatal(err)
	}

	assertRecordSetsGolden(t, client, t.Name()+"_upsert", zoneID, zoneName)

	// DELETE in yet another case removes it.
	if err := changeRecordSets(ctx, client, zoneID, aRecordChange(types.ChangeActionDelete, "wWw."+upperZoneName+".", 600, "192.0.2.2")); err != nil {
		t.Fatal(err)
	}

	assertRecordSetsGolden(t, client, t.Name()+"_delete", zoneID, zoneName)
}
