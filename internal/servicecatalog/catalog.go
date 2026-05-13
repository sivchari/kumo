// Package servicecatalog normalizes AWS, SDK, and kumo-internal service names.
package servicecatalog

import (
	"fmt"
	"sort"
	"strings"
)

// Identity describes one AWS service name as exposed to kumo control APIs.
type Identity struct {
	Canonical string   `json:"name"`
	KumoName  string   `json:"kumoName,omitempty"`
	Aliases   []string `json:"aliases,omitempty"`
	Protocols []string `json:"protocols,omitempty"`
}

// Catalog normalizes AWS SDK, AWS protocol, and kumo-internal service names.
type Catalog struct {
	byCanonical map[string]Identity
	byAlias     map[string]string
}

// New creates an empty catalog.
func New() *Catalog {
	return &Catalog{
		byCanonical: make(map[string]Identity),
		byAlias:     make(map[string]string),
	}
}

// NewDefault returns the catalog used by kumo control-plane features.
func NewDefault() *Catalog {
	c := New()

	for _, identity := range defaultIdentities() {
		if err := c.Register(&identity); err != nil {
			panic(err)
		}
	}

	return c
}

// Register adds a service identity and all aliases.
func (c *Catalog) Register(identity *Identity) error {
	identity.Canonical = canonicalKey(identity.Canonical)
	if identity.Canonical == "" {
		return fmt.Errorf("canonical service name is required")
	}

	if identity.KumoName == "" {
		identity.KumoName = identity.Canonical
	}

	aliases := make([]string, 0, 2+len(identity.Aliases))
	aliases = append(aliases, identity.Canonical, identity.KumoName)
	aliases = append(aliases, identity.Aliases...)
	identity.Aliases = uniqueStrings(identity.Aliases)
	identity.Protocols = uniqueStrings(identity.Protocols)

	if existing, ok := c.byCanonical[identity.Canonical]; ok {
		return fmt.Errorf("service %q is already registered as %q", identity.Canonical, existing.Canonical)
	}

	for _, alias := range aliases {
		key := canonicalKey(alias)
		if key == "" {
			continue
		}

		if owner, ok := c.byAlias[key]; ok && owner != identity.Canonical {
			return fmt.Errorf("service alias %q is already registered for %q", alias, owner)
		}
	}

	c.byCanonical[identity.Canonical] = *identity

	for _, alias := range aliases {
		key := canonicalKey(alias)
		if key != "" {
			c.byAlias[key] = identity.Canonical
		}
	}

	return nil
}

// Normalize returns the canonical service name for name.
func (c *Catalog) Normalize(name string) (string, bool) {
	canonical, ok := c.byAlias[canonicalKey(name)]

	return canonical, ok
}

// MustNormalize returns the canonical service name or the normalized input when unknown.
func (c *Catalog) MustNormalize(name string) string {
	if canonical, ok := c.Normalize(name); ok {
		return canonical
	}

	return canonicalKey(name)
}

// Services returns all registered services sorted by canonical name.
func (c *Catalog) Services() []Identity {
	out := make([]Identity, 0, len(c.byCanonical))
	for _, identity := range c.byCanonical {
		out = append(out, identity)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Canonical < out[j].Canonical
	})

	return out
}

func canonicalKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, " ", "")

	return s
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}

		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		out = append(out, value)
	}

	sort.Strings(out)

	return out
}

func defaultIdentities() []Identity {
	return []Identity{
		{Canonical: "acm", KumoName: "acm", Aliases: []string{"CertificateManager"}, Protocols: []string{"json"}},
		{Canonical: "amp", KumoName: "amp", Aliases: []string{"aps"}},
		{Canonical: "athena", KumoName: "athena", Aliases: []string{"AmazonAthena"}, Protocols: []string{"json"}},
		{Canonical: "cloudformation", KumoName: "cloudformation", Aliases: []string{"CloudFormation"}, Protocols: []string{"query"}},
		{Canonical: "cloudtrail", KumoName: "cloudtrail", Aliases: []string{"CloudTrail_20131101"}, Protocols: []string{"json"}},
		{Canonical: "cloudwatch", KumoName: "monitoring", Aliases: []string{"GraniteServiceVersion20100801"}, Protocols: []string{"cbor"}},
		{Canonical: "cloudwatchlogs", KumoName: "logs", Aliases: []string{"Logs_20140328"}, Protocols: []string{"json"}},
		{Canonical: "codeguruprofiler", KumoName: "codeguru-profiler", Aliases: []string{"codeguru-profiler"}},
		{Canonical: "codegurureviewer", KumoName: "codeguru-reviewer", Aliases: []string{"codeguru-reviewer"}},
		{Canonical: "cognitoidentityprovider", KumoName: "cognito-idp", Aliases: []string{"cognito", "cognito-idp", "AWSCognitoIdentityProviderService"}, Protocols: []string{"json"}},
		{Canonical: "configservice", KumoName: "configservice", Aliases: []string{"StarlingDoveService"}, Protocols: []string{"json"}},
		{Canonical: "costexplorer", KumoName: "ce", Aliases: []string{"ce", "AWSInsightsIndexService"}, Protocols: []string{"json"}},
		{Canonical: "directoryservice", KumoName: "ds", Aliases: []string{"ds", "DirectoryService_20150416"}, Protocols: []string{"json"}},
		{Canonical: "docdb", KumoName: "docdb", Aliases: []string{"documentdb"}, Protocols: []string{"query"}},
		{Canonical: "dynamodb", KumoName: "dynamodb", Aliases: []string{"DynamoDB_20120810"}, Protocols: []string{"json"}},
		{Canonical: "ec2", KumoName: "ec2", Aliases: []string{"AmazonEC2"}, Protocols: []string{"query"}},
		{Canonical: "ecr", KumoName: "ecr", Aliases: []string{"AmazonEC2ContainerRegistry_V20150921"}, Protocols: []string{"json"}},
		{Canonical: "ecs", KumoName: "ecs", Aliases: []string{"AmazonEC2ContainerServiceV20141113"}, Protocols: []string{"json"}},
		{Canonical: "elasticache", KumoName: "elasticache", Aliases: []string{"AmazonElastiCacheV9"}, Protocols: []string{"query"}},
		{Canonical: "elasticbeanstalk", KumoName: "elasticbeanstalk", Aliases: []string{"ElasticBeanstalk"}, Protocols: []string{"query"}},
		{Canonical: "elasticloadbalancingv2", KumoName: "elasticloadbalancingv2", Aliases: []string{"elbv2", "ElasticLoadBalancing"}, Protocols: []string{"query"}},
		{Canonical: "eventbridge", KumoName: "events", Aliases: []string{"events", "cloudwatch-events", "AWSEvents"}, Protocols: []string{"json"}},
		{Canonical: "firehose", KumoName: "firehose", Aliases: []string{"Firehose_20150804"}, Protocols: []string{"json"}},
		{Canonical: "forecast", KumoName: "forecast", Aliases: []string{"AmazonForecast"}, Protocols: []string{"json"}},
		{Canonical: "gamelift", KumoName: "gamelift", Aliases: []string{"GameLift"}, Protocols: []string{"json"}},
		{Canonical: "globalaccelerator", KumoName: "globalaccelerator", Aliases: []string{"GlobalAccelerator_V20180706"}, Protocols: []string{"json"}},
		{Canonical: "glue", KumoName: "glue", Aliases: []string{"AWSGlue"}, Protocols: []string{"json"}},
		{Canonical: "kinesis", KumoName: "kinesis", Aliases: []string{"Kinesis_20131202"}, Protocols: []string{"json"}},
		{Canonical: "kms", KumoName: "kms", Aliases: []string{"TrentService"}, Protocols: []string{"json"}},
		{Canonical: "memorydb", KumoName: "memorydb", Aliases: []string{"AmazonMemoryDB"}, Protocols: []string{"json"}},
		{Canonical: "neptune", KumoName: "neptune", Aliases: []string{"AmazonNeptuneDataService"}, Protocols: []string{"query"}},
		{Canonical: "organizations", KumoName: "organizations", Aliases: []string{"AWSOrganizationsV20161128"}, Protocols: []string{"json"}},
		{Canonical: "pinpointsmsvoicev2", KumoName: "pinpointsmsvoicev2", Aliases: []string{"PinpointSMSVoiceV2"}, Protocols: []string{"json"}},
		{Canonical: "rds", KumoName: "rds", Aliases: []string{"rds"}, Protocols: []string{"query"}},
		{Canonical: "redshift", KumoName: "redshift", Aliases: []string{"RedshiftServiceVersion20121201"}, Protocols: []string{"query"}},
		{Canonical: "rekognition", KumoName: "rekognition", Aliases: []string{"RekognitionService"}, Protocols: []string{"json"}},
		{Canonical: "route53resolver", KumoName: "route53resolver", Aliases: []string{"Route53Resolver"}, Protocols: []string{"json"}},
		{Canonical: "s3", KumoName: "s3", Protocols: []string{"rest"}},
		{Canonical: "sagemaker", KumoName: "sagemaker", Aliases: []string{"SageMaker"}, Protocols: []string{"json"}},
		{Canonical: "secretsmanager", KumoName: "secretsmanager", Protocols: []string{"json"}},
		{Canonical: "servicequotas", KumoName: "service-quotas", Aliases: []string{"service-quotas", "ServiceQuotasV20190624"}, Protocols: []string{"json"}},
		{Canonical: "sfn", KumoName: "states", Aliases: []string{"states", "stepfunctions", "step-functions", "AWSStepFunctions"}, Protocols: []string{"json"}},
		{Canonical: "sns", KumoName: "sns", Aliases: []string{"AmazonSimpleNotificationService"}, Protocols: []string{"query"}},
		{Canonical: "sqs", KumoName: "sqs", Aliases: []string{"AmazonSQS"}, Protocols: []string{"json"}},
		{Canonical: "ssm", KumoName: "ssm", Aliases: []string{"AmazonSSM"}, Protocols: []string{"json"}},
		{Canonical: "sts", KumoName: "sts", Aliases: []string{"AWSSecurityTokenServiceV20110615"}, Protocols: []string{"query"}},
	}
}
