package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Log          LogConfig          `mapstructure:"log"`
	Auth         AuthConfig         `mapstructure:"auth"`
	MessageQueue MessageQueueConfig `mapstructure:"message_queue"`
}

type MessageQueueConfig struct {
	Enabled  bool           `mapstructure:"enabled"`
	Type     string         `mapstructure:"type"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
}

type RabbitMQConfig struct {
	URL         string `mapstructure:"url"`
	VirtualHost string `mapstructure:"virtual_host"`
	Exchange    string `mapstructure:"exchange"`
	RoutingKey  string `mapstructure:"routing_key"`
	QueueName   string `mapstructure:"queue_name"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type AuthConfig struct {
	Users []UserConfig `mapstructure:"users"`
}

type UserConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
