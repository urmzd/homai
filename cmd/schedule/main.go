package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/urmzd/sunrise-lamp/pkg"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
)

type ScheduleEvent struct {
	TargetBrightness int `json:"target_brightness"`
}

type SunAPIResponse struct {
	Results struct {
		Sunrise string `json:"sunrise"`
		Sunset  string `json:"sunset"`
		CivilTwilightBegin string `json:"civil_twilight_begin"`
		CivilTwilightEnd   string `json:"civil_twilight_end"`
	} `json:"results"`
	Status string `json:"status"`
}

func handler(ctx context.Context, event events.CloudWatchEvent) error {
	var scheduleEvent ScheduleEvent
	err := json.Unmarshal(event.Detail, &scheduleEvent)
	if err != nil {
		return err
	}

	pkg.MqttServer = os.Getenv("MQTT_SERVER")
	pkg.MqttUsername = os.Getenv("MQTT_USERNAME")
	pkg.MqttPassword = os.Getenv("MQTT_PASSWORD")
	pkg.DeviceFriendlyName = os.Getenv("DEVICE_FRIENDLY_NAME")

	client := pkg.NewClient(pkg.MqttServer, pkg.MqttUsername, pkg.MqttPassword)
	currentBrightness := pkg.GetCurrentBrightness(client, pkg.DeviceFriendlyName)

	// Get sunrise and sunset times
	lat := os.Getenv("LAT")
	lng := os.Getenv("LNG")
	sunAPIURL := fmt.Sprintf("https://api.sunrise-sunset.org/json?lat=%s&lng=%s&timezone=UTC&date=today", lat, lng)
	sunAPIResponse := &SunAPIResponse{}

	resp, err := http.Get(sunAPIURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(sunAPIResponse); err != nil {
		return err
	}

	// Schedule events
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := eventbridge.New(sess)

	// Calculate the duration and interval between events
	sunrise, _ := time.Parse(time.RFC3339, sunAPIResponse.Results.Sunrise)
	sunset, _ := time.Parse(time.RFC3339, sunAPIResponse.Results.Sunset)
	dawn, _ := time.Parse(time.RFC3339, sunAPIResponse.Results.CivilTwilightBegin)
	dusk, _ := time.Parse(time.RFC3339, sunAPIResponse.Results.CivilTwilightEnd)
	sunriseDuration := sunrise.Sub(dawn)
	sunsetDuration := dusk.Sub(sunset)

	interval := (sunriseDuration + sunsetDuration) / time.Duration(scheduleEvent.TargetBrightness-currentBrightness)

	for i := currentBrightness + 1; i <= scheduleEvent.TargetBrightness; i++ {
		targetTime := dawn.Add(time.Duration(i) * interval)
		ruleName := fmt.Sprintf("increase-brightness-%d", i)

		putRuleInput := &eventbridge.PutRuleInput{
			Name:               aws.String(ruleName),
			ScheduleExpression: aws.String(fmt.Sprintf("rate(%d minutes)", int(interval.Minutes()))),
			State:              aws.String("ENABLED"),
		}
		_, err := svc.PutRule(putRuleInput)
		if err != nil {
			return err
		}

		putTargetsInput := &eventbridge.PutTargetsInput{
			Rule: aws.String(ruleName),
			Targets: []*eventbridge.Target{
				{
					Arn: aws.String(os.Getenv("INCREASE_LAMBDA_ARN")),
					Id:  aws.String("1"),
					Input: aws.String(fmt.Sprintf(`{
					"target_brightness": %d
					}`, i)),
				},
			},
		}
		_, err = svc.PutTargets(putTargetsInput)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
