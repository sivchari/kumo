package cloudfront

// AWS-managed policies exist in every account without being created, so a
// distribution may reference them although kumo never stores them. Only the
// IDs AWS documents are listed; the name is kept for readers.
//
// References:
//   - https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-cache-policies.html
//   - https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-response-headers-policies.html
var (
	// managedCachePolicies keeps each policy's TTLs, in seconds, in the
	// same entry as its name. ttls is nil for the Amplify Hosting policies,
	// whose TTLs are not documented.
	managedCachePolicies = map[string]managedCachePolicy{
		"2e54312d-136d-493c-8eb9-b001f22f67d2": {name: "Managed-Amplify", ttls: &cachePolicyTTLs{min: 2, def: 2, max: 600}},
		"4135ea2d-6df8-44a3-9df3-4b5a84be39ad": {name: "Managed-CachingDisabled", ttls: &cachePolicyTTLs{min: 0, def: 0, max: 0}},
		"658327ea-f89d-4fab-a63d-7e88639e58f6": {name: "Managed-CachingOptimized", ttls: &cachePolicyTTLs{min: 1, def: 86400, max: 31536000}},
		"b2884449-e4de-46a7-ac36-70bc7f1ddd6d": {name: "Managed-CachingOptimizedForUncompressedObjects", ttls: &cachePolicyTTLs{min: 1, def: 86400, max: 31536000}},
		"08627262-05a9-4f76-9ded-b50ca2e3a84f": {name: "Managed-Elemental-MediaPackage", ttls: &cachePolicyTTLs{min: 0, def: 86400, max: 31536000}},
		"83da9c7e-98b4-4e11-a168-04f0df8e2c65": {name: "UseOriginCacheControlHeaders", ttls: &cachePolicyTTLs{min: 0, def: 0, max: 31536000}},
		"4cc15a8a-d715-48a4-82b8-cc0b614638fe": {name: "UseOriginCacheControlHeaders-QueryStrings", ttls: &cachePolicyTTLs{min: 0, def: 0, max: 31536000}},

		// AWS marks the Amplify Hosting policies as used by Amplify only.
		"4d1d2f1d-3a71-49ad-9e08-7ea5d843a556": {name: "Amplify-Default"},
		"a6bad946-36c3-4c33-aa98-362c74a7fb13": {name: "Amplify-DefaultNoCookies"},
		"1c6db51a-a33f-469a-8245-dae26771f530": {name: "Amplify-ImageOptimization"},
		"7e5fad67-ee98-4ad0-b05a-394999eefc1a": {name: "Amplify-StaticContent"},
	}

	managedResponseHeadersPolicies = map[string]string{
		"5cc3b908-e619-4b99-88e5-2cf7f45965bd": "Managed-CORS-With-Preflight",
		"e61eb60c-9c35-4d20-a928-2b84e02af89c": "Managed-CORS-and-SecurityHeadersPolicy",
		"eaab4381-ed33-4a86-88ca-d9558dc6cd63": "Managed-CORS-with-preflight-and-SecurityHeadersPolicy",
		"67f7725c-6f97-4210-82d7-5512b31e9d03": "Managed-SecurityHeadersPolicy",
		"60669652-455b-4ae9-85a4-c4c02393f86c": "Managed-SimpleCORS",
	}
)

// managedCachePolicy is an entry of managedCachePolicies.
type managedCachePolicy struct {
	name string
	ttls *cachePolicyTTLs
}
