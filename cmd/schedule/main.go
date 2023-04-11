package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/urmzd/sunrise-lamp/pkg"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
	scheduler_types "github.com/aws/aws-sdk-go-v2/service/scheduler/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SunAPIResponse struct {
	Results struct {
		Sunrise            string `json:"sunrise"`
		Sunset             string `json:"sunset"`
		CivilTwilightBegin string `json:"civil_twilight_begin"`
		CivilTwilightEnd   string `json:"civil_twilight_end"`
	} `json:"results"`
	Status string `json:"status"`
}

func handler(ctx context.Context, event events.CloudWatchEvent) error {
	cfg, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		return err
	}

	ssmClient := secretsmanager.NewFromConfig(cfg)

	server := os.Getenv("SERVER")
	device := os.Getenv("DEVICE")
	increaseFunctionArn := os.Getenv("INCREASE_FUNCTION_ARN")
	credsArn := os.Getenv("CREDS")
	bucket := os.Getenv("CONFIG_BUCKET_NAME")

	pkg.MqttServer = server
	pkg.DeviceFriendlyName = device

	mqttCredentials, err := pkg.GetMqttCredentials(ssmClient, ctx, credsArn)

	if err != nil {
		return err
	}

	pkg.MqttUser = mqttCredentials.User
	pkg.MqttPassword = mqttCredentials.Password

	client := pkg.NewClient(pkg.MqttServer, pkg.MqttUser, pkg.MqttPassword)
	currentBrightness := pkg.GetCurrentBrightness(client, pkg.DeviceFriendlyName)

	s3Client := s3.NewFromConfig(cfg)
	lampConfig, err := pkg.LoadConfig(s3Client, ctx, bucket, "config.json")

	if err != nil {
		return err
	}

	apiUrl := fmt.Sprintf("https://api.sunrise-sunset.org/json?lat=%f&lng=%f&timezone=UTC&date=today", lampConfig.Location.Lat, lampConfig.Location.Long)
	sunAPIResponse := &SunAPIResponse{}

	resp, err := http.Get(apiUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(sunAPIResponse); err != nil {
		return err
	}

	schedulerClient := scheduler.NewFromConfig(cfg)

	// Calculate the duration and interval between events
	sunrise, _ := time.Parse(time.RFC3339, sunAPIResponse.Results.Sunrise)
	dawn, _ := time.Parse(time.RFC3339, sunAPIResponse.Results.CivilTwilightBegin)
	sunset, _ := time.Parse(time.RFC3339, sunAPIResponse.Results.Sunset)
	dusk, _ := time.Parse(time.RFC3339, sunAPIResponse.Results.CivilTwilightEnd)

	sunriseDuration := sunrise.Sub(dawn)
	sunsetDuration := dusk.Sub(sunset)

	if lampConfig.Sunrise.Set {
		sunriseInterval := sunriseDuration / (time.Duration(lampConfig.Sunrise.TargetBrightness - currentBrightness))

		sunriseEventName := "sunrise"
		sunriseEventDesc := "Sunrise"
		sunriseTimeZone := "UTC"
		sunriseEventSchedule := fmt.Sprintf("rate(%d minutes)", int(math.Ceil(sunriseInterval.Minutes())))
		_, err := schedulerClient.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
			Name:                       &sunriseEventName,
			Description:                &sunriseEventDesc,
			State:                      "ENABLED",
			ScheduleExpression:         &sunriseEventSchedule,
			ScheduleExpressionTimezone: &sunriseTimeZone,
			StartDate:                  &dawn,
			EndDate:                    &sunrise,
			Target: &scheduler_types.Target{
				Arn: &increaseFunctionArn,
			},
		})

		if err != nil {
			return err
		}
	}

	if lampConfig.Sunset.Set {
		sunsetInterval := sunsetDuration / (time.Duration(lampConfig.Sunset.TargetBrightness - currentBrightness))

		sunsetEventName := "sunset"
		sunsetEventDesc := "Sunset"
		sunsetTimeZone := "UTC"
		sunsetEventSchedule := fmt.Sprintf("rate(%d minutes)", int(math.Ceil((sunsetInterval.Minutes()))))
		_, err := schedulerClient.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
			Name:                       &sunsetEventName,
			Description:                &sunsetEventDesc,
			State:                      "ENABLED",
			ScheduleExpression:         &sunsetEventSchedule,
			ScheduleExpressionTimezone: &sunsetTimeZone,
			StartDate:                  &sunset,
			EndDate:                    &dusk,
			Target: &scheduler_types.Target{
				Arn: &increaseFunctionArn,
			},
		})

		if err != nil {
			return err
		}
	}

	return nil

}

func main() {
	lambda.Start(handler)
}
