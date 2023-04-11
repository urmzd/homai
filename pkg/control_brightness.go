package pkg

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
	MqttServer          string
	MqttUsername        string
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

var (
	setCmd = &cobra.Command{
		Use:   "set [value]",
		Short: "Set brightness",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			value, _ := strconv.Atoi(args[0])
			client:= NewClient(MqttServer, MqttUsername, MqttPassword)
			SetBrightness(client, DeviceFriendlyName, value)
		},
	}

	getCmd = &cobra.Command{
		Use:   "get",
		Short: "Get current brightness",
		Run: func(cmd *cobra.Command, args []string) {
			client := NewClient(MqttServer, MqttUsername, MqttPassword)
			GetCurrentBrightness(client, DeviceFriendlyName)
		},
	}
)

func init() {
	RootCmd.AddCommand(setCmd)
	RootCmd.AddCommand(getCmd)
}

func SetBrightness(client mqtt.Client, device string, brightness int) {
	scaledBrightness := scaleBrightness(brightness)
	message := make(map[string]int)
	message["brightness"] = scaledBrightness
	client.Publish("zigbee2mqtt/"+device+"/set", 0, false, toJSON(message))
	client.Disconnect(250)
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


func GetCurrentBrightness(client mqtt.Client, device string) {
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

var RootCmd = &cobra.Command{
	Use:   "sunlight-lamp",
	Short: "Control Sylvania A19 lightbulb using MQTT",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		MqttServer, _ = cmd.Flags().GetString("mqtt-server")
		MqttUsername, _ = cmd.Flags().GetString("mqtt-username")
		MqttPassword, _ = cmd.Flags().GetString("mqtt-password")
		DeviceFriendlyName, _ = cmd.Flags().GetString("device-friendly-name")
	},
}
