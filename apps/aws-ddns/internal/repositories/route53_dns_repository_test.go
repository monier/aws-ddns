package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRoute53 struct {
	listOutput  *route53.ListResourceRecordSetsOutput
	listErr     error
	listInput   *route53.ListResourceRecordSetsInput
	changeErr   error
	changeInput *route53.ChangeResourceRecordSetsInput
}

func (f *fakeRoute53) ListResourceRecordSets(_ context.Context, params *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	f.listInput = params
	return f.listOutput, f.listErr
}

func (f *fakeRoute53) ChangeResourceRecordSets(_ context.Context, params *route53.ChangeResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error) {
	f.changeInput = params
	return &route53.ChangeResourceRecordSetsOutput{}, f.changeErr
}

func aRecordSet(name, value string) types.ResourceRecordSet {
	return types.ResourceRecordSet{
		Name:            aws.String(name),
		Type:            types.RRTypeA,
		ResourceRecords: []types.ResourceRecord{{Value: aws.String(value)}},
	}
}

func TestReadARecordReturnsExistingValue(t *testing.T) {
	fake := &fakeRoute53{listOutput: &route53.ListResourceRecordSetsOutput{
		// Route 53 returns fully-qualified names with a trailing dot.
		ResourceRecordSets: []types.ResourceRecordSet{aRecordSet("ddns-test.example.com.", "203.0.113.7")},
	}}
	repository := NewRoute53DNSRepository(fake, "ZONE1")

	value, exists, err := repository.ReadARecord(context.Background(), "ddns-test.example.com")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "203.0.113.7", value)
	assert.Equal(t, "ZONE1", aws.ToString(fake.listInput.HostedZoneId))
}

func TestReadARecordReportsMissingWhenZoneReturnsNothing(t *testing.T) {
	fake := &fakeRoute53{listOutput: &route53.ListResourceRecordSetsOutput{}}
	repository := NewRoute53DNSRepository(fake, "ZONE1")

	_, exists, err := repository.ReadARecord(context.Background(), "ddns-test.example.com")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestReadARecordReportsMissingWhenListReturnsTheNextRecord(t *testing.T) {
	fake := &fakeRoute53{listOutput: &route53.ListResourceRecordSetsOutput{
		ResourceRecordSets: []types.ResourceRecordSet{aRecordSet("zzz.example.com.", "192.0.2.1")},
	}}
	repository := NewRoute53DNSRepository(fake, "ZONE1")

	_, exists, err := repository.ReadARecord(context.Background(), "ddns-test.example.com")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestReadARecordWrapsAPIErrors(t *testing.T) {
	fake := &fakeRoute53{listErr: errors.New("access denied")}
	repository := NewRoute53DNSRepository(fake, "ZONE1")

	_, _, err := repository.ReadARecord(context.Background(), "ddns-test.example.com")

	assert.ErrorContains(t, err, "list resource record sets")
}

func TestUpsertARecordSendsAnUpsertChange(t *testing.T) {
	fake := &fakeRoute53{}
	repository := NewRoute53DNSRepository(fake, "ZONE1")

	err := repository.UpsertARecord(context.Background(), "ddns-test.example.com", "203.0.113.7", 60)

	require.NoError(t, err)
	require.NotNil(t, fake.changeInput)
	assert.Equal(t, "ZONE1", aws.ToString(fake.changeInput.HostedZoneId))
	require.Len(t, fake.changeInput.ChangeBatch.Changes, 1)
	change := fake.changeInput.ChangeBatch.Changes[0]
	assert.Equal(t, types.ChangeActionUpsert, change.Action)
	assert.Equal(t, "ddns-test.example.com", aws.ToString(change.ResourceRecordSet.Name))
	assert.Equal(t, types.RRTypeA, change.ResourceRecordSet.Type)
	assert.Equal(t, int64(60), aws.ToInt64(change.ResourceRecordSet.TTL))
	require.Len(t, change.ResourceRecordSet.ResourceRecords, 1)
	assert.Equal(t, "203.0.113.7", aws.ToString(change.ResourceRecordSet.ResourceRecords[0].Value))
}

func TestUpsertARecordWrapsAPIErrors(t *testing.T) {
	fake := &fakeRoute53{changeErr: errors.New("throttled")}
	repository := NewRoute53DNSRepository(fake, "ZONE1")

	err := repository.UpsertARecord(context.Background(), "ddns-test.example.com", "203.0.113.7", 60)

	assert.ErrorContains(t, err, "change resource record sets")
}
