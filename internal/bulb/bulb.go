package bulb

import (
	"context"
	"fmt"
	"path"
	"strconv"

	"github.com/eclipse/paho.golang/paho"
)

type Dimmer struct {
	client *MQTTClient
	dimmer *paho.Publish
}

// NewDimmer creates a new instance that can dim the bulb at the given client.
func NewDimmer(c *MQTTClient) Dimmer {
	return Dimmer{
		client: c,
		dimmer: &paho.Publish{
			Topic: path.Join(c.Topic, "cmnd", "Dimmer"),
			QoS:   1,
		},
	}
}

// SetBlocking sets whether or not Dim should wait for an ACK.
func (d Dimmer) SetBlocking(blocking bool) {
	if blocking {
		d.dimmer.QoS = 1
	} else {
		d.dimmer.QoS = 0
	}
}

// Dim dims the bulb.
func (d Dimmer) Dim(ctx context.Context, intensity int) error {
	if intensity < 0 || intensity > 100 {
		return fmt.Errorf("intensity %d out of range", intensity)
	}

	d.dimmer.Payload = strconv.AppendInt(d.dimmer.Payload[:0], int64(intensity), 10)

	_, err := d.client.Publish(ctx, d.dimmer)
	if err != nil {
		return err
	}

	return nil
}
