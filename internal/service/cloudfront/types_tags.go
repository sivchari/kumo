package cloudfront

import "encoding/xml"

// DistributionConfigWithTags is the CreateDistributionWithTags request body.
type DistributionConfigWithTags struct {
	XMLName            xml.Name                  `xml:"DistributionConfigWithTags"`
	DistributionConfig CreateDistributionRequest `xml:"DistributionConfig"`
	Tags               TagsXML                   `xml:"Tags"`
}

// TagsXML is the Tags structure used by CreateDistributionWithTags, the
// TagResource request and the ListTagsForResource response.
type TagsXML struct {
	XMLName xml.Name    `xml:"Tags"`
	Xmlns   string      `xml:"xmlns,attr,omitempty"`
	Items   TagItemsXML `xml:"Items"`
}

// TagItemsXML is the Items list of TagsXML.
type TagItemsXML struct {
	Tags []TagXML `xml:"Tag"`
}

// TagXML is a single key/value tag.
type TagXML struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// TagKeysXML is the UntagResource request body.
type TagKeysXML struct {
	XMLName xml.Name `xml:"TagKeys"`
	Items   struct {
		Keys []string `xml:"Key"`
	} `xml:"Items"`
}
