package service

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/viper"

	// Load viper/remote so that we can get configurations from Consul
	_ "github.com/spf13/viper/remote"
)

// Configuration defines the interface for configuration objects
type Configuration interface {
	GetString(key string) string

	IsSet(key string) bool

	LoadConfiguration(cfgFile string) error
}

// NewConfiguration returns a new Configuration instance
func NewConfiguration() Configuration {
	return &concreteConfig{}
}

// concreteConfig implements the Configuration interface
type concreteConfig struct {
	*viper.Viper
}

func (c concreteConfig) GetString(key string) string {
	return c.Viper.GetString(key)
}

func (c concreteConfig) IsSet(key string) bool {
	return c.Viper.IsSet(key)
}

// LoadConfiguration loads the configuration for the application from different configuration sources
func (c concreteConfig) LoadConfiguration(cfgFile string) error {
	log.Debug("Reading configuration ...")

	// From the environment
	c.SetEnvPrefix("PROVISION")
	c.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	c.AutomaticEnv()

	if cfgFile != "" {
		log.Debug(
			fmt.Sprintf(
				"Reading configuration from: %s",
				cfgFile))

		c.SetConfigFile(cfgFile)
	}

	if err := c.ReadInConfig(); err != nil {
		log.Fatal(
			fmt.Sprintf(
				"Configuration invalid. Error was %v",
				err))
	}

	// Only use consul if we have a host+port and consul key specified
	if c.IsSet("consul.enabled") && c.GetBool("consul.enabled") {
		c.loadFromConsul()
	}

	return nil
}

func (c concreteConfig) loadFromConsul() {

	c.SetConfigType("yaml")

	consulHost := c.GetString("consul.host")
	consulPort := c.GetInt("consul.port")
	consulKeyPath := c.GetString("consul.keyPath")
	log.Debug(
		fmt.Sprintf(
			"Reading configuration from Consul on host %s:%d via key %s.",
			consulHost,
			consulPort,
			consulKeyPath))

	if err := c.AddRemoteProvider("consul", fmt.Sprintf("%s:%d", consulHost, consulPort), consulKeyPath); err != nil {
		log.Fatal(
			fmt.Sprintf(
				"Unable to connect to Consul at host %s:%d to read key %s. Error was %v",
				consulHost,
				consulPort,
				consulKeyPath,
				err))
	}

	if err := c.ReadRemoteConfig(); err != nil {
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
			if err := c.WatchRemoteConfig(); err != nil {
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
			c.ReadRemoteConfig()
		}
	}
}
