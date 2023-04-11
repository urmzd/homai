import * as cdk from "aws-cdk-lib";
import * as s3 from "aws-cdk-lib/aws-s3";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as secretsmanager from "aws-cdk-lib/aws-secretsmanager";
import * as elbv2 from "aws-cdk-lib/aws-elasticloadbalancingv2";
import { Construct } from "constructs";
import * as path from "node:path";
import * as iam from "aws-cdk-lib/aws-iam";
import * as events from "aws-cdk-lib/aws-events";
import * as lambda from "aws-cdk-lib/aws-lambda";
import { LambdaFunction } from "aws-cdk-lib/aws-events-targets";

export class SunriseLampStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const vpc = new ec2.Vpc(this, "VPC", {
      maxAzs: 2,
      subnetConfiguration: [
        {
          cidrMask: 24,
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
          name: "Private",
        },
        {
          cidrMask: 24,
          subnetType: ec2.SubnetType.PUBLIC,
          name: "Public",
        }
       ],
     });

    vpc.addGatewayEndpoint("S3Endpoint", {
      service: ec2.GatewayVpcEndpointAwsService.S3,
      subnets: [
        {
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        },
      ],
    });

    vpc.addInterfaceEndpoint("SecretsManagerEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.SECRETS_MANAGER,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("KmsEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.KMS,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("ECREndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.ECR,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("ECRDockerEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.ECR_DOCKER,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("ECSEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.ECS,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("SsmEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.SSM,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("SsmMessageEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.SSM_MESSAGES,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("EcsLogsEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.ECS_TELEMETRY,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("CloudWatchLogsEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.CLOUDWATCH_LOGS,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
    });

    vpc.addInterfaceEndpoint("EC2MessagesEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.EC2_MESSAGES,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        onePerAz: true,
      }),
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
          user: "root",
        }),
        generateStringKey: "password",
      },
    });

    const taskRole = new iam.Role(this, "TaskRole", {
      assumedBy: new iam.ServicePrincipal("ecs-tasks.amazonaws.com"),
    });

    taskRole.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName(
        "service-role/AmazonECSTaskExecutionRolePolicy"
      )
    );

    taskRole.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName("AmazonSSMManagedInstanceCore")
    );

    const taskExecutionRole = new iam.Role(this, "TaskExecutionRole", {
      assumedBy: new iam.ServicePrincipal("ecs-tasks.amazonaws.com"),
    });

    const taskDefinition = new ecs.FargateTaskDefinition(
      this,
      "MQTTBrokerTask",
      {
        taskRole: taskRole,

        executionRole: taskExecutionRole,
        memoryLimitMiB: 512,
        cpu: 256,
      }
    );

    const imagePath = path.resolve(process.cwd(), "../configs/mosquitto");
    console.log(imagePath);

    taskDefinition
      .addContainer("MQTTBrokerContainer", {
        cpu: 256,
        memoryLimitMiB: 512,
        secrets: {
          MQTT_USER: ecs.Secret.fromSecretsManager(mqttCreds, "user"),
          MQTT_PASSWORD: ecs.Secret.fromSecretsManager(mqttCreds, "password"),
        },
        image: ecs.ContainerImage.fromAsset(imagePath),
        containerName: "mqtt",
        logging: new ecs.AwsLogDriver({
          streamPrefix: "mqtt-broker",
        }),
      })
      .addPortMappings({
        containerPort: 1883,
        hostPort: 1883,
        protocol: ecs.Protocol.TCP,
      });

    const mqttBroker = new ecs.FargateService(this, "MQTTBrokerService", {
      cluster,
      taskDefinition,
      enableExecuteCommand: true,
      assignPublicIp: false,
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      },
    });

    // Create an Application Load Balancer
    const mqttLoadBalancer = new elbv2.NetworkLoadBalancer(
      this,
      "MQTTLoadBalancer",
      {
        vpc,
        internetFacing: false,
      }
    );

    // Add a listener to the Application Load Balancer
    const mqttListener = mqttLoadBalancer.addListener("MQTTListener", {
      port: 1883,
      protocol: elbv2.Protocol.TCP,
    });


    // Add the MQTT broker as a target for the listener
    mqttListener.addTargets("MQTTBrokerTarget", {
      port: 1883,
      targets: [mqttBroker],
      healthCheck: {
        path: "/",
        port: "8080",
      }
    });

    const coreEnv = {
      BUCKET: configBucket.bucketName,
      SERVER: `mqtt://${mqttLoadBalancer.loadBalancerDnsName}:1883`,
      CREDS: mqttCreds.secretArn,
      DEVICE: "a19",
    };

    const lambdasPath = lambda.Code.fromAsset(
      path.resolve(process.cwd(), "../bin")
    );

    // We call this function several times as scheduled by the schedule lambda.
    const increaseFunction = new lambda.Function(
      this,
      "LambdaIncreaseFunction",
      {
        runtime: lambda.Runtime.GO_1_X,
        handler: "increase",
        code: lambdasPath,
        environment: coreEnv,
        vpc: vpc,
        vpcSubnets: {
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        },
      }
    );

    // This is scheduled every day at midnight, and schedules multiple increases for sunrise and sunset.
    // We number the invocations to prevent redundant logic from running.
    const scheduleFunction = new lambda.Function(
      this,
      "LambdaScheduleFunction",
      {
        runtime: lambda.Runtime.GO_1_X,
        handler: "schedule",
        code: lambdasPath,
        environment: {
          ...coreEnv,
          INCREASE_FUNCTION_ARN: increaseFunction.functionArn,
        },
        vpc: vpc,
        vpcSubnets: {
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
        },
      }
    );

    // Define EventBridge rule for midnight schedule
    const midnightRule = new events.Rule(this, "MidnightSchedule", {
      schedule: events.Schedule.rate(cdk.Duration.days(1)),
    });

    // Add Lambda function as a target for the EventBridge rule
    midnightRule.addTarget(new LambdaFunction(scheduleFunction));

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
