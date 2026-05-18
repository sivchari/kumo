{
  "Listeners": [
    {
      "AlpnPolicy": null,
      "Certificates": null,
      "DefaultActions": [
        {
          "Type": "forward",
          "AuthenticateCognitoConfig": null,
          "AuthenticateOidcConfig": null,
          "FixedResponseConfig": null,
          "ForwardConfig": null,
          "JwtValidationConfig": null,
          "Order": null,
          "RedirectConfig": null,
          "TargetGroupArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:targetgroup/test-listener-tg/63d12692-5e22-467"
        }
      ],
      "ListenerArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:listener/app/test-listener-lb/94d275ff-a173-4c4/28ad3876-2ae6-4d3",
      "LoadBalancerArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:loadbalancer/app/test-listener-lb/94d275ff-a173-4c4",
      "MutualAuthentication": null,
      "Port": 80,
      "Protocol": "HTTP",
      "SslPolicy": null
    }
  ],
  "ResultMetadata": {}
}
