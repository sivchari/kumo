[
  {
    "Case": "RequestCertificate empty key",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "ImportCertificate empty key",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "AddTagsToCertificate empty key",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "RemoveTagsFromCertificate empty key",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "AddTagsToCertificate 129 character key",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "AddTagsToCertificate 257 character value",
    "Error": {
      "Code": "ValidationException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "AddTagsToCertificate 128 character key and 256 character value"
  },
  {
    "Case": "RequestCertificate reserved key",
    "Error": {
      "Code": "InvalidTagException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "ImportCertificate reserved key",
    "Error": {
      "Code": "InvalidTagException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "AddTagsToCertificate reserved key",
    "Error": {
      "Code": "InvalidTagException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "RemoveTagsFromCertificate reserved key",
    "Error": {
      "Code": "InvalidTagException",
      "HTTPStatus": 400
    }
  },
  {
    "Case": "RequestCertificate 51 tags",
    "Error": {
      "Code": "TooManyTagsException",
      "HTTPStatus": 400
    }
  }
]