package cloudfront

// AWS-managed policies exist in every account without being created, so a
// distribution may reference them although kumo never stores them. Only the
// IDs AWS documents are listed; the name is kept for readers.
//
// References:
//   - https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-cache-policies.html
//   - https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-response-headers-policies.html
var (
	managedCachePolicies = map[string]string{
		"2e54312d-136d-493c-8eb9-b001f22f67d2": "Managed-Amplify",
		"4135ea2d-6df8-44a3-9df3-4b5a84be39ad": "Managed-CachingDisabled",
		"658327ea-f89d-4fab-a63d-7e88639e58f6": "Managed-CachingOptimized",
		"b2884449-e4de-46a7-ac36-70bc7f1ddd6d": "Managed-CachingOptimizedForUncompressedObjects",
		"08627262-05a9-4f76-9ded-b50ca2e3a84f": "Managed-Elemental-MediaPackage",
	}

	managedResponseHeadersPolicies = map[string]string{
		"5cc3b908-e619-4b99-88e5-2cf7f45965bd": "Managed-CORS-With-Preflight",
		"e61eb60c-9c35-4d20-a928-2b84e02af89c": "Managed-CORS-and-SecurityHeadersPolicy",
		"eaab4381-ed33-4a86-88ca-d9558dc6f027": "Managed-CORS-with-preflight-and-SecurityHeadersPolicy",
		"67f7725c-6f97-4210-82d7-5512b31e9d03": "Managed-SecurityHeadersPolicy",
		"60669652-455b-4ae9-85a4-c4c02393f86c": "Managed-SimpleCORS",
	}
)
