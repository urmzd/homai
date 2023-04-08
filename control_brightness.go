package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	setCmd = &cobra.Command{
		Use:   "set [value]",
		Short: "Set brightness",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			value, _ := strconv.Atoi(args[0])
			client:= NewClient(mqttServer, mqttUsername, mqttPassword)
			controlBrightness(client, deviceFriendlyName, value)
		},
	}

	getCmd = &cobra.Command{
		Use:   "get",
		Short: "Get current brightness",
		Run: func(cmd *cobra.Command, args []string) {
			client := NewClient(mqttServer, mqttUsername, mqttPassword)
			getCurrentBrightness(client, deviceFriendlyName)
		},
	}
)

func init() {
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(getCmd)
}

func setBrightness(client mqtt.Client, device string, brightness int) {
	scaledBrightness := scaleBrightness(brightness)
	message := make(map[string]int)
	message["brightness"] = scaledBrightness
	client.Publish("zigbee2mqtt/"+device+"/set", 0, false, toJSON(message))
}

func toJSON(obj interface{}) string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func scaleBrightness(value int) int {
	return int(float64(value) * (255.0 / 10.0))
}

func NewClient(server string, user string, pass string) mqtt.Client {
	opts := mqtt.NewClientOptions().AddBroker(server).SetUsername(user).SetPassword(pass)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()

	return client
}

func controlBrightness(client mqtt.Client, device string, value int) {
	token := client.Connect()
	token.Wait()
	setBrightness(client, device, value)

	client.Disconnect(250)
}

func getCurrentBrightness(client mqtt.Client, device string) {
	var brightness int
	messageChannel := make(chan mqtt.Message)

	token := client.Subscribe("zigbee2mqtt/"+device, 0, func(client mqtt.Client, msg mqtt.Message) {
		messageChannel <- msg
	})
	token.Wait()

	if token.Error() != nil {
		log.Error().Msgf("Error subscribing to topic: %v", token.Error())
		return
	}

	getPayload := map[string]string{
		"brightness": "",
	}
	client.Publish("zigbee2mqtt/"+device+"/get", 0, false, toJSON(getPayload))

	select {
	case msg := <-messageChannel:
		var deviceData map[string]interface{}
		if err := json.Unmarshal(msg.Payload(), &deviceData); err != nil {
			log.Error().Msgf("Error unmarshalling device data: %v", err)
			return
		}
		log.Info().Msgf("Device data: %v", deviceData)
		if b, ok := deviceData["brightness"].(float64); ok {
			brightness = int(b)
		}
	case <-time.After(5 * time.Second):
		log.Info().Msg("Timeout waiting for brightness")
		return
	}

	client.Unsubscribe("zigbee2mqtt/" + device)

	fmt.Printf("%d", brightness)
}
