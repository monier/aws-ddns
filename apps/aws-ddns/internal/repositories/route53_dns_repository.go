package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// Route53DNSRepository reads and writes the managed A record in one Route 53
// hosted zone.
type Route53DNSRepository struct {
	client       Route53API
	hostedZoneID string
}

func NewRoute53DNSRepository(client Route53API, hostedZoneID string) *Route53DNSRepository {
	return &Route53DNSRepository{client: client, hostedZoneID: hostedZoneID}
}

// ReadARecord returns the A record's current value and whether it exists.
func (r *Route53DNSRepository) ReadARecord(ctx context.Context, name string) (string, bool, error) {
	out, err := r.client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(r.hostedZoneID),
		StartRecordName: aws.String(name),
		StartRecordType: types.RRTypeA,
		MaxItems:        aws.Int32(1),
	})
	if err != nil {
		return "", false, fmt.Errorf("list resource record sets: %w", err)
	}

	if len(out.ResourceRecordSets) == 0 {
		return "", false, nil
	}

	record := out.ResourceRecordSets[0]
	// ListResourceRecordSets starts AT the requested name/type but returns whatever
	// follows it in the zone when the record itself does not exist.
	if record.Type != types.RRTypeA || !sameRecordName(aws.ToString(record.Name), name) {
		return "", false, nil
	}
	if len(record.ResourceRecords) == 0 {
		return "", false, nil
	}
	return aws.ToString(record.ResourceRecords[0].Value), true, nil
}

// UpsertARecord creates or updates the A record with the given IPv4 and TTL.
func (r *Route53DNSRepository) UpsertARecord(ctx context.Context, name string, ipv4 string, ttl int64) error {
	_, err := r.client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.hostedZoneID),
		ChangeBatch: &types.ChangeBatch{
			Comment: aws.String("aws-ddns automatic synchronization"),
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(name),
						Type: types.RRTypeA,
						TTL:  aws.Int64(ttl),
						ResourceRecords: []types.ResourceRecord{
							{Value: aws.String(ipv4)},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("change resource record sets: %w", err)
	}
	return nil
}

// sameRecordName compares record names ignoring case and the trailing dot
// Route 53 appends to fully-qualified names.
func sameRecordName(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}
