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
      "LoadBalancerArns": [],
      "Matcher": {
        "GrpcCode": null,
        "HttpCode": "200"
      },
      "Port": 80,
      "Protocol": "HTTP",
      "ProtocolVersion": null,
      "TargetControlPort": null,
      "TargetGroupArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:targetgroup/test-describe-tg/4e7cadf9-54af-432",
      "TargetGroupName": "test-describe-tg",
      "TargetType": "instance",
      "UnhealthyThresholdCount": 2,
      "VpcId": "vpc-12345678"
    }
  ],
  "ResultMetadata": {}
}