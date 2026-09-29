package sns

import "testing"

func TestUnsubscribeURL_DerivesRegionFromARN(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		arn  string
		want string
	}{
		{
			name: "region from subscription ARN",
			arn:  "arn:aws:sns:ap-northeast-1:000000000000:my-topic:2bcfbf39-05c3-41de-beaa-fcfcc21c8f55",
			want: "https://sns.ap-northeast-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:ap-northeast-1:000000000000:my-topic:2bcfbf39-05c3-41de-beaa-fcfcc21c8f55",
		},
		{
			name: "unparsable ARN falls back to the default region",
			arn:  "not-an-arn",
			want: "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=not-an-arn",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := unsubscribeURL(tc.arn); got != tc.want {
				t.Errorf("unsubscribeURL(%q) = %q, want %q", tc.arn, got, tc.want)
			}
		})
	}
}
