package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	pkg "github.com/urmzd/sunrise-lamp/pkg"
)

type IncreaseEvent struct {
	TargetBrightness int `json:"target_brightness"`
}

func handler(ctx context.Context, event events.CloudWatchEvent) error {
	var increaseEvent IncreaseEvent
	err := json.Unmarshal(event.Detail, &increaseEvent)
	if err != nil {
		return err
	}

	pkg.MqttServer = os.Getenv("MQTT_SERVER")
	pkg.MqttUser = os.Getenv("MQTT_USER")
	pkg.MqttPassword = os.Getenv("MQTT_PASSWORD")
	pkg.DeviceFriendlyName = os.Getenv("DEVICE_FRIENDLY_NAME")

	client := pkg.NewClient(pkg.MqttServer, pkg.MqttUser, pkg.MqttPassword)
	currentBrightness := pkg.GetCurrentBrightness(client, pkg.DeviceFriendlyName)

	if currentBrightness < increaseEvent.TargetBrightness {
		pkg.SetBrightness(client, pkg.DeviceFriendlyName, currentBrightness+1)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
