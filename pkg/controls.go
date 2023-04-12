package pkg

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)


func SetBrightness(client mqtt.Client, device string, brightness int) {
	scaledBrightness := scaleBrightness(brightness)
	message := make(map[string]int)
	message["brightness"] = scaledBrightness
	token := client.Publish("zigbee2mqtt/"+device+"/set", 0, false, toJSON(message))
	token.Wait()
	if token.Error() != nil {
		log.Error().Msgf("Error publishing to topic: %v", token.Error())
	}
}


func GetCurrentBrightness(client mqtt.Client, device string) int {
	var brightness int
	messageChannel := make(chan mqtt.Message)

	token := client.Subscribe("zigbee2mqtt/"+device, 0, func(client mqtt.Client, msg mqtt.Message) {
		messageChannel <- msg
	})
	token.Wait()

	if token.Error() != nil {
		log.Error().Msgf("Error subscribing to topic: %v", token.Error())
		return 0
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
			return 0
		}
		log.Info().Msgf("Device data: %v", deviceData)
		if b, ok := deviceData["brightness"].(float64); ok {
			brightness = int(b)
		}
	case <-time.After(5 * time.Second):
		log.Info().Msg("Timeout waiting for brightness")
		return 0
	}

	client.Unsubscribe("zigbee2mqtt/" + device)

	fmt.Printf("%d", brightness)
	return brightness
}

func scaleBrightness(value int) int {
	return int(float64(value) * (255.0 / 10.0))
}

func toJSON(obj interface{}) string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(bytes)
}
