[
  {
    "Case": "DescribeCertificate unknown ARN",
    "Error": {
      "Code": "ResourceNotFoundException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "GetCertificate unknown ARN",
    "Error": {
      "Code": "ResourceNotFoundException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "DeleteCertificate unknown ARN",
    "Error": {
      "Code": "ResourceNotFoundException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "GetCertificate pending certificate",
    "Error": {
      "Code": "RequestInProgressException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "ListTagsForCertificate unknown ARN",
    "Error": {
      "Code": "ResourceNotFoundException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "AddTagsToCertificate unknown ARN",
    "Error": {
      "Code": "ResourceNotFoundException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "RemoveTagsFromCertificate unknown ARN",
    "Error": {
      "Code": "ResourceNotFoundException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "ListTagsForCertificate empty ARN",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "AddTagsToCertificate empty tags",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "RemoveTagsFromCertificate empty tags",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  }
]