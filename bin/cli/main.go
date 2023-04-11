package main

import (
	"github.com/spf13/cobra"
	"github.com/urmzd/sunrise-lamp/pkg"
)

func main() {
	pkg.RootCmd.PersistentFlags().StringVar(&pkg.MqttServer, "mqtt-server", "mqtt://localhost:1883", "MQTT server address")
	pkg.RootCmd.PersistentFlags().StringVar(&pkg.MqttUsername, "mqtt-username", "root", "MQTT username")
	pkg.RootCmd.PersistentFlags().StringVar(&pkg.MqttPassword, "mqtt-password", "pass", "MQTT password")
	pkg.RootCmd.PersistentFlags().StringVar(&pkg.DeviceFriendlyName, "device-friendly-name", "a19", "Device friendly name")

	cobra.CheckErr(pkg.RootCmd.Execute())
}
