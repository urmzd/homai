package pkg


import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	MqttServer          string
	MqttUser        string
	MqttPassword        string
	DeviceFriendlyName  string
)

func NewClient(server string, user string, pass string) mqtt.Client {
	opts := mqtt.NewClientOptions().AddBroker(server).SetUsername(user).SetPassword(pass)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()

	return client
}
