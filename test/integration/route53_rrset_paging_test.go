//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
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
// records you requested", while the page itself stops at 300. Both pages are
// compared in order against golden files, so a missing, repeated or
// misordered record set fails.
func TestRoute53_ListResourceRecordSetsMaxItemsAbovePageLimit(t *testing.T) {
	t.Parallel()

	const (
		zoneName  = "paging-limit-test.example.com"
		stored    = 301
		requested = 1000
	)

	// Stored in reverse, so the pages show the listing order rather than the
	// order of creation.
	names := make([]string, 0, stored)
	for i := stored - 1; i >= 0; i-- {
		names = append(names, fmt.Sprintf("r%03d.%s.", i, zoneName))
	}

	client := newRoute53Client(t)
	zoneID := createZoneWithARecords(t, client, zoneName, names)

	page, err := client.ListResourceRecordSets(t.Context(), &route53.ListResourceRecordSetsInput{
		HostedZoneId: zoneID,
		MaxItems:     aws.Int32(requested),
	})
	if err != nil {
		t.Fatal(err)
	}

	// golden ignores the order of arrays by default; the order is part of
	// what this test checks.
	golden.New(t, golden.WithIgnoreOrder(false)).Assert(t.Name()+"_first_page", newRecordSetPage(page))

	rest, err := client.ListResourceRecordSets(t.Context(), &route53.ListResourceRecordSetsInput{
		HostedZoneId:    zoneID,
		MaxItems:        aws.Int32(requested),
		StartRecordName: page.NextRecordName,
		StartRecordType: page.NextRecordType,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreOrder(false)).Assert(t.Name()+"_second_page", newRecordSetPage(rest))
}

// recordSetPage is a ListResourceRecordSets page as the paging tests compare
// it: the record sets in order as "name type", and the paging fields.
type recordSetPage struct {
	RecordSets           []string
	IsTruncated          bool
	MaxItems             int32
	NextRecordName       string
	NextRecordType       types.RRType
	NextRecordIdentifier string
}

func newRecordSetPage(out *route53.ListResourceRecordSetsOutput) recordSetPage {
	return recordSetPage{
		RecordSets:           recordSetKeys(out.ResourceRecordSets),
		IsTruncated:          out.IsTruncated,
		MaxItems:             aws.ToInt32(out.MaxItems),
		NextRecordName:       aws.ToString(out.NextRecordName),
		NextRecordType:       out.NextRecordType,
		NextRecordIdentifier: aws.ToString(out.NextRecordIdentifier),
	}
}

// The reference sorts by the reversed name and notes that the trailing dot
// "can change the sort order when the record name contains characters that
// appear before . (decimal 46) in the ASCII table", listing
// ! " # $ % & ' ( ) * + , -. A label continued by any of them sorts before
// the same label ending there.
func TestRoute53_ListResourceRecordSetsTrailingDotOrder(t *testing.T) {
	t.Parallel()

	const zoneName = "sort-order-test.example.com"

	names := []string{"www." + zoneName + "."}
	for _, c := range `!"#$%&'()*+,-` {
		names = append(names, "www"+string(c)+"1."+zoneName+".")
	}

	client := newRoute53Client(t)
	zoneID := createZoneWithARecords(t, client, zoneName, names)

	out, err := client.ListResourceRecordSets(t.Context(), &route53.ListResourceRecordSetsInput{HostedZoneId: zoneID})
	if err != nil {
		t.Fatal(err)
	}

	// golden ignores the order of arrays by default; the order is what this
	// test checks.
	golden.New(t, golden.WithIgnoreOrder(false)).Assert(t.Name(), recordSetKeys(out.ResourceRecordSets))
}

// createZoneWithARecords creates a hosted zone holding an A record set for
// each of names and deletes both when the test ends. The caller reference is
// unique per run.
func createZoneWithARecords(t *testing.T, client *route53.Client, zoneName string, names []string) *string {
	t.Helper()

	zone, err := client.CreateHostedZone(t.Context(), &route53.CreateHostedZoneInput{
		Name:            aws.String(zoneName),
		CallerReference: aws.String(fmt.Sprintf("test-%s-%d", zoneName, time.Now().UnixNano())),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.ChangeResourceRecordSets(context.Background(), &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: zone.HostedZone.Id,
			ChangeBatch:  &types.ChangeBatch{Changes: aRecordChanges(types.ChangeActionDelete, names)},
		})
		_, _ = client.DeleteHostedZone(context.Background(), &route53.DeleteHostedZoneInput{Id: zone.HostedZone.Id})
	})

	if _, err := client.ChangeResourceRecordSets(t.Context(), &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: zone.HostedZone.Id,
		ChangeBatch:  &types.ChangeBatch{Changes: aRecordChanges(types.ChangeActionCreate, names)},
	}); err != nil {
		t.Fatal(err)
	}

	return zone.HostedZone.Id
}

// aRecordChanges returns a change with the given action for each name, each
// for an A record set pointing at 192.0.2.1.
func aRecordChanges(action types.ChangeAction, names []string) []types.Change {
	changes := make([]types.Change, 0, len(names))
	for _, name := range names {
		changes = append(changes, types.Change{
			Action: action,
			ResourceRecordSet: &types.ResourceRecordSet{
				Name:            aws.String(name),
				Type:            types.RRTypeA,
				TTL:             aws.Int64(300),
				ResourceRecords: []types.ResourceRecord{{Value: aws.String("192.0.2.1")}},
			},
		})
	}

	return changes
}

// recordSetKeys renders record sets in order as "name type".
func recordSetKeys(sets []types.ResourceRecordSet) []string {
	keys := make([]string, 0, len(sets))
	for _, rrset := range sets {
		keys = append(keys, aws.ToString(rrset.Name)+" "+string(rrset.Type))
	}

	return keys
}
