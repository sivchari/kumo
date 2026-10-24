//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
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

// A MaxItems above the 300 record sets the reference says one call returns is
// echoed back, since the reference defines MaxItems as "The maximum number of
// records you requested", while the page itself stops at 300.
func TestRoute53_ListResourceRecordSetsMaxItemsAbovePageLimit(t *testing.T) {
	t.Parallel()

	const (
		pageLimit = 300
		stored    = pageLimit + 1
		requested = 1000
	)

	client := newRoute53Client(t)
	ctx := t.Context()

	zone, err := client.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String("paging-limit-test.example.com"),
		CallerReference: aws.String("test-list-record-sets-paging-limit"),
	})
	if err != nil {
		t.Fatal(err)
	}

	changes := func(action types.ChangeAction) []types.Change {
		out := make([]types.Change, 0, stored)
		for i := range stored {
			out = append(out, types.Change{
				Action: action,
				ResourceRecordSet: &types.ResourceRecordSet{
					Name:            aws.String(fmt.Sprintf("r%03d.paging-limit-test.example.com.", i)),
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

	page, err := client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: zone.HostedZone.Id,
		MaxItems:     aws.Int32(requested),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(page.ResourceRecordSets) != pageLimit || !page.IsTruncated || aws.ToInt32(page.MaxItems) != requested {
		t.Fatalf("first page: got %d records IsTruncated=%v MaxItems=%d, want %d records IsTruncated=true MaxItems=%d",
			len(page.ResourceRecordSets), page.IsTruncated, aws.ToInt32(page.MaxItems), pageLimit, requested)
	}

	wantNext := fmt.Sprintf("r%03d.paging-limit-test.example.com.", pageLimit)
	if aws.ToString(page.NextRecordName) != wantNext || page.NextRecordType != types.RRTypeA {
		t.Fatalf("next: got (%q, %q), want (%q, A)", aws.ToString(page.NextRecordName), page.NextRecordType, wantNext)
	}

	rest, err := client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId:    zone.HostedZone.Id,
		MaxItems:        aws.Int32(requested),
		StartRecordName: page.NextRecordName,
		StartRecordType: page.NextRecordType,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(rest.ResourceRecordSets) != 1 || rest.IsTruncated || aws.ToString(rest.ResourceRecordSets[0].Name) != wantNext {
		t.Errorf("second page: got %d records IsTruncated=%v, want only %s", len(rest.ResourceRecordSets), rest.IsTruncated, wantNext)
	}
}
