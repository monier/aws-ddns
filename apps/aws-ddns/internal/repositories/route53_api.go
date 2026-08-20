package repositories

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// Route53API is the narrow seam of the AWS SDK Route 53 client this app uses,
// satisfied by *route53.Client and by test fakes.
type Route53API interface {
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(ctx context.Context, params *route53.ChangeResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error)
}
