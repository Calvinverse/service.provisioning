package config

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/viper"

	// Load viper/remote so that we can get configurations from Consul
	_ "github.com/spf13/viper/remote"
)

// LoadConfig loads the configuration for the application from different configuration sources
func LoadConfig(cfgFile string) {

	log.Debug("Reading configuration ...")

	// From the environment
	viper.SetEnvPrefix("PROVISION")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		log.Debug(
			fmt.Sprintf(
				"Reading configuration from: %s",
				cfgFile))

		viper.SetConfigFile(cfgFile)
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(
			fmt.Sprintf(
				"Configuration invalid. Error was %v",
				err))
	}

	// Only use consul if we have a host+port and consul key specified
	if viper.IsSet("consul.enabled") && viper.GetBool("consul.enabled") {
		loadFromConsul()
	}
}

func loadFromConsul() {

	viper.SetConfigType("yaml")

	consulHost := viper.GetString("consul.host")
	consulPort := viper.GetInt("consul.port")
	consulKeyPath := viper.GetString("consul.keyPath")
	log.Debug(
		fmt.Sprintf(
			"Reading configuration from Consul on host %s:%d via key %s.",
			consulHost,
			consulPort,
			consulKeyPath))

	if err := viper.AddRemoteProvider("consul", fmt.Sprintf("%s:%d", consulHost, consulPort), consulKeyPath); err != nil {
		log.Fatal(
			fmt.Sprintf(
				"Unable to connect to Consul at host %s:%d to read key %s. Error was %v",
				consulHost,
				consulPort,
				consulKeyPath,
				err))
	}

	if err := viper.ReadRemoteConfig(); err != nil {
		log.Warn(
			fmt.Sprintf(
				"Unable to read the configuration from Consul at key %s via host %s:%d. Error was %v",
				consulKeyPath,
				consulHost,
				consulPort,
				err))
	}

	// see: https://github.com/spf13/viper/issues/326
	listenerCh := make(chan bool)

	go func() {
		for {
			if err := viper.WatchRemoteConfig(); err != nil {
				log.Errorf("unable to read remote config: %v", err)
				continue
			}

			for {
				time.Sleep(time.Second * 5) // delay after each request
				listenerCh <- true
			}
		}
	}()

	for {
		select {
		case <-listenerCh:
			fmt.Println("rereading remote config!")
			viper.ReadRemoteConfig()
		}
	}
}