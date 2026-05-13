package servicecatalog

import "testing"

func TestDefaultCatalogNormalizesAWSAndKumoServiceNames(t *testing.T) {
	catalog := NewDefault()

	tests := map[string]string{
		"eventbridge":                   "eventbridge",
		"events":                        "eventbridge",
		"cloudwatch-events":             "eventbridge",
		"sfn":                           "sfn",
		"states":                        "sfn",
		"step_functions":                "sfn",
		"cloudwatch":                    "cloudwatch",
		"monitoring":                    "cloudwatch",
		"cloudwatchlogs":                "cloudwatchlogs",
		"logs":                          "cloudwatchlogs",
		"elasticloadbalancingv2":        "elasticloadbalancingv2",
		"elbv2":                         "elasticloadbalancingv2",
		"cognitoidentityprovider":       "cognitoidentityprovider",
		"cognito-idp":                   "cognitoidentityprovider",
		"directoryservice":              "directoryservice",
		"ds":                            "directoryservice",
		"costexplorer":                  "costexplorer",
		"ce":                            "costexplorer",
		"codeguru-profiler":             "codeguruprofiler",
		"codeguruprofiler":              "codeguruprofiler",
		"AmazonSQS":                     "sqs",
		"DynamoDB_20120810":             "dynamodb",
		"GraniteServiceVersion20100801": "cloudwatch",
	}

	for input, want := range tests {
		got, ok := catalog.Normalize(input)
		if !ok {
			t.Fatalf("Normalize(%q) returned !ok", input)
		}

		if got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCatalogRejectsAliasCollisions(t *testing.T) {
	catalog := New()
	if err := catalog.Register(&Identity{Canonical: "s3", KumoName: "s3"}); err != nil {
		t.Fatal(err)
	}

	err := catalog.Register(&Identity{Canonical: "sqs", Aliases: []string{"s3"}})
	if err == nil {
		t.Fatal("expected alias collision error")
	}
}
