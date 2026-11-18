[
  {
    "Step": "request 50 tags with 49 keys",
    "Tags": 49
  },
  {
    "Step": "add a 50th key",
    "Tags": 50
  },
  {
    "Step": "add a 51st key",
    "Error": {
      "Code": "TooManyTagsException",
      "HTTPStatus": 400
    },
    "Tags": 50
  },
  {
    "Step": "overwrite a key at the quota",
    "Tags": 50
  },
  {
    "Step": "add 50 keys in one call",
    "Tags": 50
  }
]