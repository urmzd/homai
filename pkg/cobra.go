package pkg

import (
	"strconv"
	"github.com/spf13/cobra"
)

var (
	setCmd = &cobra.Command{
		Use:   "set [value]",
		Short: "Set brightness",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			value, _ := strconv.Atoi(args[0])
			client:= NewClient(MqttServer, MqttUser, MqttPassword)
			SetBrightness(client, DeviceFriendlyName, value)
		},
	}

	getCmd = &cobra.Command{
		Use:   "get",
		Short: "Get current brightness",
		Run: func(cmd *cobra.Command, args []string) {
			client := NewClient(MqttServer, MqttUser, MqttPassword)
			GetCurrentBrightness(client, DeviceFriendlyName)
		},
	}
)

func init() {
	RootCmd.AddCommand(setCmd)
	RootCmd.AddCommand(getCmd)
}

var RootCmd = &cobra.Command{
	Use:   "sunlight-lamp",
	Short: "Control Sylvania A19 lightbulb using MQTT",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		MqttServer, _ = cmd.Flags().GetString("mqtt-server")
		MqttUser, _ = cmd.Flags().GetString("mqtt-user")
		MqttPassword, _ = cmd.Flags().GetString("mqtt-password")
		DeviceFriendlyName, _ = cmd.Flags().GetString("device-friendly-name")
	},
}
