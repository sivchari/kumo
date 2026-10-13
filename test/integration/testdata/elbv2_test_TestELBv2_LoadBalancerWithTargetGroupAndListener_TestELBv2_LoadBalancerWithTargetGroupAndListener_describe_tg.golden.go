{
  "NextMarker": null,
  "TargetGroups": [
    {
      "HealthCheckEnabled": true,
      "HealthCheckIntervalSeconds": 30,
      "HealthCheckPath": "/",
      "HealthCheckPort": "traffic-port",
      "HealthCheckProtocol": "HTTP",
      "HealthCheckTimeoutSeconds": 5,
      "HealthyThresholdCount": 5,
      "IpAddressType": "",
      "LoadBalancerArns": [
        "arn:aws:elasticloadbalancing:us-east-1:000000000000:loadbalancer/app/test-full-lb/7bffbb8c-4e44-42d"
      ],
      "Matcher": {
        "GrpcCode": null,
        "HttpCode": "200"
      },
      "Port": 80,
      "Protocol": "HTTP",
      "ProtocolVersion": null,
      "TargetControlPort": null,
      "TargetGroupArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:targetgroup/test-full-tg/4c43749b-89ca-45e",
      "TargetGroupName": "test-full-tg",
      "TargetType": "instance",
      "UnhealthyThresholdCount": 2,
      "VpcId": "vpc-12345678"
    }
  ],
  "ResultMetadata": {}
}