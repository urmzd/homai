package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	pkg "github.com/urmzd/sunrise-lamp/pkg"
)

type ControlEvent struct {
	Name  string `json:"name"`
	Level int `json:"level"`
}

type DeviceMapping struct {
	Name       string `json:"Name"`
	DeviceName string `json:"DeviceName"`
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	lambda.Start(handler)
}

func getDeviceName(ctx context.Context, name string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	dbClient := dynamodb.NewFromConfig(cfg)

	tableName := os.Getenv("DEVICE_MAPPING_TABLE")

	result, err := dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &tableName,
		Key: map[string]types.AttributeValue{
			"Name": &types.AttributeValueMemberS{
				Value: name,
			},
		},
	})

	if err != nil {
		return "", err
	}

	if result.Item == nil {
		log.Error().Err(err).Msg("No item found for name")
		return "", err
	}

	var deviceMapping DeviceMapping
	err = attributevalue.UnmarshalMap(result.Item, &deviceMapping)
	if err != nil {
		return "", err
	}

	return deviceMapping.DeviceName, nil
}

func handler(ctx context.Context, event events.CloudWatchEvent) error {
	var controlEvent ControlEvent
	err := json.Unmarshal(event.Detail, &controlEvent)
	if err != nil {
		return err
	}

	deviceName, err := getDeviceName(ctx, controlEvent.Name)
	if err != nil {
		return err
	}

	pkg.MqttServer = os.Getenv("SERVER")
	pkg.MqttUser = os.Getenv("USER")
	pkg.MqttPassword = os.Getenv("PASSWORD")
	pkg.DeviceFriendlyName = deviceName

	client := pkg.NewClient(pkg.MqttServer, pkg.MqttUser, pkg.MqttPassword)
	currentBrightness := pkg.GetCurrentBrightness(client, pkg.DeviceFriendlyName)

	pkg.SetBrightness(client, pkg.DeviceFriendlyName, currentBrightness+1)

	return nil
}
