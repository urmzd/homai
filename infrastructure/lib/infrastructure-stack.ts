import * as cdk from "aws-cdk-lib";
import * as s3 from "aws-cdk-lib/aws-s3";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as events from "aws-cdk-lib/aws-events";
import * as targets from "aws-cdk-lib/aws-events-targets";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as secretsmanager from "aws-cdk-lib/aws-secretsmanager";
import { Construct } from "constructs";

export class InfrastructureStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // Define VPC with a private and a public subnet
    const vpc = new ec2.Vpc(this, "VPC", {
      maxAzs: 2,
      subnetConfiguration: [
        {
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
          name: "Private",
        },
        {
          subnetType: ec2.SubnetType.PUBLIC,
          name: "Public",
        },
      ],
    });

    // Define S3 bucket for the configuration file
    const configBucket = new s3.Bucket(this, "ConfigBucket");

    // Define ECS cluster and service in the private subnet
    const cluster = new ecs.Cluster(this, "MQTTBrokerCluster", {
      vpc: vpc,
    });

    const mqttCreds = new secretsmanager.Secret(this, "MqttCreds", {
      generateSecretString: {
        secretStringTemplate: JSON.stringify({
          username: "root",
        }),
        generateStringKey: "password",
      },
    });

    const taskDefinition = new ecs.FargateTaskDefinition(
      this,
      "MQTTBrokerTask"
    );

    taskDefinition
      .addContainer("MQTTBrokerContainer", {
        image: ecs.ContainerImage.fromAsset("../"),
        environment: {
          MQTT_USERNAME: mqttCreds.secretValueFromJson("username").toString(),
          MQTT_PASSWORD: mqttCreds.secretValueFromJson("password").toString(),
        },
        logging: new ecs.AwsLogDriver({
          streamPrefix: "mqtt-broker",
        }),
      })
      .addPortMappings({
        containerPort: 1883,
      });

    new ecs.FargateService(this, "MQTTBrokerService", {
      cluster,
      taskDefinition,
      assignPublicIp: false,
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      },
    });

    // Define Lambda functions
    const increaseFunction = new lambda.Function(
      this,
      "LambdaIncreaseFunction",
      {
        runtime: lambda.Runtime.GO_1_X,
        handler: "increase",
        code: lambda.Code.fromAsset("../"),
        environment: {
          CONFIG_BUCKET_NAME: configBucket.bucketName,
        },
        vpc: vpc,
        vpcSubnets: {
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        },
      }
    );

    const scheduleFunction = new lambda.Function(
      this,
      "LambdaScheduleFunction",
      {
        runtime: lambda.Runtime.GO_1_X,
        handler: "schedule",
        code: lambda.Code.fromAsset("../"),
        environment: {
          INCREASE_FUNCTION_ARN: increaseFunction.functionArn,
        },
        vpc: vpc,
        vpcSubnets: {
          subnetType: ec2.SubnetType.PUBLIC,
        },
      }
    );

    // Define EventBridge rule for midnight schedule
    const midnightRule = new events.Rule(this, "MidnightSchedule", {
      schedule: events.Schedule.expression("cron(0 0 * * ? *)"),
    });

    // Add Lambda function as a target for the EventBridge rule
    midnightRule.addTarget(new targets.LambdaFunction(scheduleFunction));

    // Grant required permissions to Lambda functions
    configBucket.grantReadWrite(increaseFunction);

    const mqttCredentials = secretsmanager.Secret.fromSecretNameV2(
      this,
      "MQTTCredentials",
      "mqtt-credentials"
    );

    mqttCredentials.grantRead(increaseFunction);
  }
}
