//go:build integration

package integration

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

// TestRoute53_ChangeBatchIsAtomic sends batches that fail on their last
// change, a duplicate CREATE, after a DELETE or UPSERT that would succeed on
// its own. The reference says Route 53 "either makes all or none of the
// changes in the change batch request", so neither batch may change the zone.
func TestRoute53_ChangeBatchIsAtomic(t *testing.T) {
	t.Parallel()

	const zoneName = "atomic-batch-test.example.com"

	www := "www." + zoneName + "."
	api := "api." + zoneName + "."

	client := newRoute53Client(t)
	ctx := t.Context()
	zoneID := createZoneWithARecords(t, client, zoneName, []string{www, api})

	duplicate := aRecordChanges(types.ChangeActionCreate, []string{api})[0]
	upsert := aRecordChanges(types.ChangeActionUpsert, []string{www})[0]
	upsert.ResourceRecordSet.ResourceRecords = []types.ResourceRecord{{Value: aws.String("192.0.2.2")}}

	for _, first := range []types.Change{
		aRecordChanges(types.ChangeActionDelete, []string{www})[0],
		upsert,
	} {
		_, err := client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: zoneID,
			ChangeBatch:  &types.ChangeBatch{Changes: []types.Change{first, duplicate}},
		})

		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("%s then duplicate CREATE: expected an API error, got %T: %v", first.Action, err, err)
		}
	}

	listed, err := client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{HostedZoneId: zoneID})
	if err != nil {
		t.Fatal(err)
	}

	records := make([]string, 0, len(listed.ResourceRecordSets))
	for _, rrset := range listed.ResourceRecordSets {
		records = append(records, fmt.Sprintf("%s %s %s", aws.ToString(rrset.Name), rrset.Type, aws.ToString(rrset.ResourceRecords[0].Value)))
	}

	zone, err := client.GetHostedZone(ctx, &route53.GetHostedZoneInput{Id: zoneID})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreOrder(false)).Assert(t.Name(), struct {
		Records                []string
		ResourceRecordSetCount int64
	}{records, aws.ToInt64(zone.HostedZone.ResourceRecordSetCount)})
}
