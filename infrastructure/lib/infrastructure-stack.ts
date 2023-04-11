import * as cdk from "aws-cdk-lib";
import * as s3 from "aws-cdk-lib/aws-s3";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as secretsmanager from "aws-cdk-lib/aws-secretsmanager";
import * as elbv2 from "aws-cdk-lib/aws-elasticloadbalancingv2";
import { Construct } from "constructs";
import * as path from "node:path"
import * as iam from "aws-cdk-lib/aws-iam";
import * as events from "aws-cdk-lib/aws-events"
import * as lambda from "aws-cdk-lib/aws-lambda"
import { LambdaFunction } from "aws-cdk-lib/aws-events-targets";

export class SunriseLampStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // Define VPC with a private and a public subnet
    const vpc = new ec2.Vpc(this, "VPC", {
      maxAzs: 2,
      subnetConfiguration: [
        {
          cidrMask: 24,
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
          name: "Private",
        },
      ],
    });

    vpc.addInterfaceEndpoint("SecretsManagerEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.SECRETS_MANAGER,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      })
    })

    vpc.addInterfaceEndpoint("ECREndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.ECR,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      })
    })

    vpc.addInterfaceEndpoint("ECSEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.ECS,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      })
    })

    vpc.addInterfaceEndpoint("KmsEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.KMS,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      })
    })

    vpc.addInterfaceEndpoint("SsmEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.SSM,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      })
    })

    vpc.addInterfaceEndpoint("SsmMessageEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.SSM_MESSAGES,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      })
    })

    vpc.addInterfaceEndpoint("EcsLogsEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.ECS_TELEMETRY,
      privateDnsEnabled: true,
      subnets: vpc.selectSubnets({
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      })
    })

    vpc.addInterfaceEndpoint("CloudWatchLogsEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.CLOUDWATCH_LOGS,
    });

    vpc.addInterfaceEndpoint("EC2MessagesEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.EC2_MESSAGES,
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

    const taskExecutionRole = new iam.Role(this, "TaskExecutionRole", {
      assumedBy: new iam.ServicePrincipal("ecs-tasks.amazonaws.com"),
    });

    taskExecutionRole.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName("service-role/AmazonECSTaskExecutionRolePolicy")
    );

    taskExecutionRole.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName("AmazonSSMManagedInstanceCore")
    );

    // TODO: update wildcard afterwards.
    taskExecutionRole.addToPolicy(
      new iam.PolicyStatement({
        actions: [
          'ecr:GetAuthorizationToken',
          'ecr:BatchCheckLayerAvailability',
          'ecr:GetDownloadUrlForLayer',
          'ecr:BatchGetImage',
          'logs:CreateLogStream',
          'logs:PutLogEvents',
          'secretsmanager:GetSecretValue',
          'kms:Decrypt',
        ],
        resources: ['*'],
      }),
    );

    const taskDefinition = new ecs.FargateTaskDefinition(
      this,
      "MQTTBrokerTask",
      {
        executionRole: taskExecutionRole,
        memoryLimitMiB: 512,
        cpu: 256,
      }
    );

    const imagePath = path.resolve(process.cwd(), "../configs/mosquitto")
    console.log(imagePath)

    taskDefinition
      .addContainer("MQTTBrokerContainer", {
        cpu: 256,
        memoryLimitMiB: 512,
        secrets: {
          MQTT_USERNAME: ecs.Secret.fromSecretsManager(mqttCreds, "username"),
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
      assignPublicIp: false,
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
      },
    });

    // Create an Application Load Balancer
    const mqttLoadBalancer = new elbv2.NetworkLoadBalancer(this, "MQTTLoadBalancer", {
      vpc,
      internetFacing: false,
    });

    // Add a listener to the Application Load Balancer
    const mqttListener = mqttLoadBalancer.addListener("MQTTListener", {
      port: 1883,
      protocol: elbv2.Protocol.TCP,
    });

    // Add the MQTT broker as a target for the listener
    mqttListener.addTargets("MQTTBrokerTarget", {
      port: 1883,
      targets: [mqttBroker],
    });

    const coreEnv = {
      BUCKET: configBucket.bucketName,
      SERVER: `mqtt://${mqttLoadBalancer.loadBalancerDnsName}:1883`,
      CREDS: mqttCreds.secretArn,
      DEVICE: "a19",
    }

    const lambdasPath = lambda.Code.fromAsset(path.resolve(process.cwd(), "../bin/x86_64"))

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
