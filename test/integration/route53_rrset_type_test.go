//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"
)

// The reference lists the valid values of ResourceRecordSet.Type. Creating or
// upserting a record set of any other type fails, and the rest of the change
// batch is not applied either.
func TestRoute53_ChangeResourceRecordSetsRejectsUnlistedType(t *testing.T) {
	t.Parallel()

	const zoneName = "unlisted-type-test.example.com"

	client := newRoute53Client(t)

	zone, err := client.CreateHostedZone(t.Context(), &route53.CreateHostedZoneInput{
		Name:            aws.String(zoneName),
		CallerReference: aws.String(fmt.Sprintf("test-%s-%d", zoneName, time.Now().UnixNano())),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{Id: zone.HostedZone.Id})
	})

	for _, action := range []types.ChangeAction{types.ChangeActionCreate, types.ChangeActionUpsert} {
		_, err := client.ChangeResourceRecordSets(t.Context(), &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: zone.HostedZone.Id,
			ChangeBatch: &types.ChangeBatch{Changes: []types.Change{
				{Action: action, ResourceRecordSet: &types.ResourceRecordSet{
					Name:            aws.String("valid." + zoneName + "."),
					Type:            types.RRTypeA,
					TTL:             aws.Int64(300),
					ResourceRecords: []types.ResourceRecord{{Value: aws.String("192.0.2.1")}},
				}},
				{Action: action, ResourceRecordSet: &types.ResourceRecordSet{
					Name:            aws.String("unlisted." + zoneName + "."),
					Type:            types.RRType("FOO"),
					TTL:             aws.Int64(300),
					ResourceRecords: []types.ResourceRecord{{Value: aws.String("192.0.2.1")}},
				}},
			}},
		})

		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "InvalidInput" {
			t.Errorf("%s with an unlisted type: got %v, want InvalidInput", action, err)
		}
	}

	out, err := client.ListResourceRecordSets(t.Context(), &route53.ListResourceRecordSetsInput{HostedZoneId: zone.HostedZone.Id})
	if err != nil {
		t.Fatal(err)
	}

	if len(out.ResourceRecordSets) != 0 {
		t.Errorf("record sets after the rejected batches: got %q, want none", recordSetKeys(out.ResourceRecordSets))
	}
}
