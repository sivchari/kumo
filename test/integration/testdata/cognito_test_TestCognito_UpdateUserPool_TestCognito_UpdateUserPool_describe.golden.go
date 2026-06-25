{
  "UserPool": {
    "AccountRecoverySetting": {
      "RecoveryMechanisms": [
        {
          "Name": "verified_email",
          "Priority": 1
        }
      ]
    },
    "AdminCreateUserConfig": {
      "AllowAdminCreateUserOnly": false,
      "InviteMessageTemplate": null,
      "UnusedAccountValidityDays": 0
    },
    "AliasAttributes": [],
    "Arn": "arn:aws:cognito-idp:us-east-1:000000000000:userpool/us-east-1_55d96b7c3",
    "AutoVerifiedAttributes": [
      "email"
    ],
    "CreationDate": "2026-06-12T07:05:37Z",
    "CustomDomain": null,
    "DeletionProtection": "INACTIVE",
    "DeviceConfiguration": null,
    "Domain": null,
    "EmailConfiguration": null,
    "EmailConfigurationFailure": null,
    "EmailVerificationMessage": null,
    "EmailVerificationSubject": null,
    "EstimatedNumberOfUsers": 0,
    "Id": "us-east-1_55d96b7c3",
    "LambdaConfig": {
      "CreateAuthChallenge": null,
      "CustomEmailSender": null,
      "CustomMessage": null,
      "CustomSMSSender": null,
      "DefineAuthChallenge": null,
      "InboundFederation": null,
      "KMSKeyID": null,
      "PostAuthentication": null,
      "PostConfirmation": null,
      "PreAuthentication": null,
      "PreSignUp": null,
      "PreTokenGeneration": null,
      "PreTokenGenerationConfig": null,
      "UserMigration": null,
      "VerifyAuthChallengeResponse": null
    },
    "LastModifiedDate": "2026-06-12T07:05:37Z",
    "MfaConfiguration": "OFF",
    "Name": "update-user-pool",
    "Policies": {
      "PasswordPolicy": {
        "MinimumLength": 8,
        "PasswordHistorySize": null,
        "RequireLowercase": true,
        "RequireNumbers": true,
        "RequireSymbols": true,
        "RequireUppercase": true,
        "TemporaryPasswordValidityDays": 7
      },
      "SignInPolicy": null
    },
    "SchemaAttributes": [
      {
        "AttributeDataType": "String",
        "DeveloperOnlyAttribute": false,
        "Mutable": false,
        "Name": "sub",
        "NumberAttributeConstraints": null,
        "Required": true,
        "StringAttributeConstraints": {
          "MaxLength": "2048",
          "MinLength": "1"
        }
      },
      {
        "AttributeDataType": "String",
        "DeveloperOnlyAttribute": false,
        "Mutable": true,
        "Name": "name",
        "NumberAttributeConstraints": null,
        "Required": false,
        "StringAttributeConstraints": {
          "MaxLength": "2048",
          "MinLength": "0"
        }
      },
      {
        "AttributeDataType": "String",
        "DeveloperOnlyAttribute": false,
        "Mutable": true,
        "Name": "email",
        "NumberAttributeConstraints": null,
        "Required": false,
        "StringAttributeConstraints": {
          "MaxLength": "2048",
          "MinLength": "0"
        }
      }
    ],
    "SmsAuthenticationMessage": null,
    "SmsConfiguration": null,
    "SmsConfigurationFailure": null,
    "SmsVerificationMessage": null,
    "Status": "Enabled",
    "UserAttributeUpdateSettings": null,
    "UserPoolAddOns": null,
    "UserPoolTags": {},
    "UserPoolTier": "ESSENTIALS",
    "UsernameAttributes": [],
    "UsernameConfiguration": null,
    "VerificationMessageTemplate": {
      "DefaultEmailOption": "CONFIRM_WITH_CODE",
      "EmailMessage": null,
      "EmailMessageByLink": null,
      "EmailSubject": null,
      "EmailSubjectByLink": null,
      "SmsMessage": null
    }
  },
  "ResultMetadata": {}
}