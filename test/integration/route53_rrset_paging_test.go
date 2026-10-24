//go:build integration

package integration

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"
)

func TestRoute53_ListResourceRecordSetsPaging(t *testing.T) {
	t.Parallel()

	client := newRoute53Client(t)
	ctx := t.Context()

	zone, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("paging-test.example.com"),
		CallerReference: aws.String("test-list-record-sets-paging"),
	})
	if err != nil {
		t.Fatal(err)
	}

	names := []string{"www.paging-test.example.com.", "api.paging-test.example.com.", "b.paging-test.example.com."}

	changes := func(action types.ChangeAction) []types.Change {
		out := make([]types.Change, 0, len(names))
		for _, name := range names {
			out = append(out, types.Change{
				Action: action,
				ResourceRecordSet: &types.ResourceRecordSet{
					Name:            aws.String(name),
					Type:            types.RRTypeA,
					TTL:             aws.Int64(300),
					ResourceRecords: []types.ResourceRecord{{Value: aws.String("192.0.2.1")}},
				},
			})
		}

		return out
	}

	t.Cleanup(func() {
		_, _ = client.ChangeResourceRecordSets(context.Background(), &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: zone.HostedZone.Id,
			ChangeBatch:  &types.ChangeBatch{Changes: changes(types.ChangeActionDelete)},
		})
		_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{Id: zone.HostedZone.Id})
	})

	if _, err := client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: zone.HostedZone.Id,
		ChangeBatch:  &types.ChangeBatch{Changes: changes(types.ChangeActionCreate)},
	}); err != nil {
		t.Fatal(err)
	}

	input := &route53.ListResourceRecordSetsInput{HostedZoneId: zone.HostedZone.Id, MaxItems: aws.Int32(1)}

	var got []string

	for range len(names) + 2 {
		page, err := client.ListResourceRecordSets(ctx, input)
		if err != nil {
			t.Fatal(err)
		}

		if len(page.ResourceRecordSets) != 1 || aws.ToInt32(page.MaxItems) != 1 {
			t.Fatalf("page: got %d records MaxItems=%d, want 1 record MaxItems=1",
				len(page.ResourceRecordSets), aws.ToInt32(page.MaxItems))
		}

		got = append(got, aws.ToString(page.ResourceRecordSets[0].Name))

		if !page.IsTruncated {
			break
		}

		input.StartRecordName = page.NextRecordName
		input.StartRecordType = page.NextRecordType
		input.StartRecordIdentifier = page.NextRecordIdentifier
	}

	want := []string{"api.paging-test.example.com.", "b.paging-test.example.com.", "www.paging-test.example.com."}
	if !slices.Equal(got, want) {
		t.Errorf("paged names: got %q, want %q", got, want)
	}

	_, err = client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId:    zone.HostedZone.Id,
		StartRecordType: types.RRTypeA,
	})

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "InvalidInput" {
		t.Errorf("type without name: got %v, want InvalidInput", err)
	}

	_, err = client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId:    zone.HostedZone.Id,
		StartRecordName: aws.String(names[0]),
		StartRecordType: types.RRType("FOO"),
	})
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "InvalidInput" {
		t.Errorf("unlisted type: got %v, want InvalidInput", err)
	}
}
