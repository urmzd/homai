# Infrastructure: 

The `cdk.json` file tells the CDK toolkit how to execute your app.

## Useful commands

 * `cdk deploy`      deploy this stack to your default AWS account/region
 * `cdk diff`        compare deployed stack with current state
 * `cdk synth`       emits the synthesized CloudFormation template
 * `go test`         run unit tests


## Overview
DESIRED COMMANDS:
- ON
- OFF
- INCREASE
- DECREASE
- SET SUNRISE 
- DELETE SUNRISE
- SET SUNSET
- DELETE SUNSET

Compute:
- Step Functions
- Lambda

Storage: 
- S3: Stores the configuration file.

Network:
- API Gateway: Protects the API + Static Content

General:
- Secrets Manager
- IOT Core
- CloudWatch Events (schedule events)

```go
type Config struct {
    Brightness: uint8 // between 0 - 10, 0 will set ON: false, and 1-10 will set ON: true
    Sunrise: {
        Set: bool,
        Start: time
        End: time
        Brightness: uint8 // Goes from current to this value.
    },
    Sunset: {
        Set: bool
        Start: time
        End: time
        Brightness: uint8 // Goes from current to this value.
    }
}
```

