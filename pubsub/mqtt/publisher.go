package mqtt

import (
	"github.com/barnybug/gohome/pubsub"

	MQTT "git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
)

// Publisher for mqtt
type Publisher struct {
	broker  string
	client  *MQTT.MqttClient
	channel chan *pubsub.Event
}

// ID of Publisher
func (pub *Publisher) ID() string {
	return "mqtt: " + pub.broker
}

// Emit an event
func (pub *Publisher) Emit(ev *pubsub.Event) {
	// put all topics under gohome/
	topic := "gohome/" + ev.Topic
	r := pub.client.Publish(MQTT.QOS_ONE, topic, ev.Bytes())
	<-r
}
