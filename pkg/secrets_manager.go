package pkg

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type MqttCredentials struct {
	User string `json:"user"`
	Password string `json:"password"`
}

func GetMqttCredentials(client *secretsmanager.Client, ctx context.Context, secretArn string) (MqttCredentials, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: &secretArn,
	}

	output, err := client.GetSecretValue(ctx, input)
	if err != nil {
		return MqttCredentials{}, err
	}

	var mqttCreds MqttCredentials
	err = json.Unmarshal([]byte(*output.SecretString), &mqttCreds)
	if err != nil {
		return MqttCredentials{}, err
	}

	return mqttCreds, nil
}
