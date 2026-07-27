{
  "AuthorizerCredentialsArn": null,
  "AuthorizerId": "1ab2bde1-f",
  "AuthorizerPayloadFormatVersion": null,
  "AuthorizerResultTtlInSeconds": null,
  "AuthorizerType": "JWT",
  "AuthorizerUri": null,
  "EnableSimpleResponses": null,
  "IdentitySource": [
    "$request.header.Authorization"
  ],
  "IdentityValidationExpression": null,
  "JwtConfiguration": {
    "Audience": [
      "test-audience"
    ],
    "Issuer": "https://issuer.example.com"
  },
  "Name": "test-jwt-authorizer-renamed",
  "ResultMetadata": {}
}