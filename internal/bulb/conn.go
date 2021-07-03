package bulb

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"

	"github.com/eclipse/paho.golang/paho"
	"github.com/pkg/errors"
)

// MQTTClient is the current MQTT client.
type MQTTClient struct {
	*paho.Client
	Topic string
}

// Connect connects to an MQTT URL.
func Connect(ctx context.Context, mqttURLString string) (*MQTTClient, error) {
	mqttURL, err := url.Parse(mqttURLString)
	if err != nil {
		log.Fatalln("failed to parse mqtt URL:", err)
	}

	var pahoConn net.Conn

	switch mqttURL.Scheme {
	case "mqtt", "tcp":
		c, err := net.Dial("tcp", mqttURL.Host)
		if err != nil {
			return nil, errors.Wrap(err, "failed to dial TCP")
		}
		pahoConn = c
	default:
		return nil, fmt.Errorf("unsupported URL scheme %q", mqttURL.Scheme)
	}

	client := paho.NewClient(paho.ClientConfig{Conn: pahoConn})
	password, hasPassword := mqttURL.User.Password()

	r, err := client.Connect(ctx, &paho.Connect{
		ClientID:     "wyzenip",
		Password:     []byte(password),
		PasswordFlag: hasPassword,
		Username:     mqttURL.User.Username(),
		UsernameFlag: mqttURL.User.Username() != "",
		KeepAlive:    60,
		CleanStart:   true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect")
	}
	if r.ReasonCode != 0 {
		return nil, fmt.Errorf("unexpected connect code %d", r.ReasonCode)
	}

	return &MQTTClient{
		Client: client,
		Topic:  strings.TrimPrefix(mqttURL.Path, "/"),
	}, nil
}

// Disconnect is a helper function that disconnects the Paho client gracefully.
func (c *MQTTClient) Disconnect() {
	c.Client.Disconnect(&paho.Disconnect{
		ReasonCode: 0,
	})
}
