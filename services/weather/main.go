// Package weather is a service to alert a daily digest of the last day's weather conditions, and
// actively alert on unusual conditions (heavy rain, strong winds).
//
// This requires the graphite service to be recording event data.
package weather

import (
	"fmt"
	"log"
	"time"

	"github.com/barnybug/gohome/lib/graphite"
	"github.com/barnybug/gohome/pubsub"
	"github.com/barnybug/gohome/services"
	"github.com/barnybug/gohome/util"
)

type td struct {
	temp float64
	noun string
}

var lowTemperatures = []td{
	td{-5, "a very cold"},
	td{-2, "a rather cold"},
	td{0, "a freezing"},
	td{2, "a frosty"},
	td{5, "a cold"},
	td{7, "a moderate"},
	td{10, "a pleasant"},
	td{15, "a hot"},
	td{25, "a scorching"},
}

var highTemperatures = []td{
	td{1, "a very cold"},
	td{4, "a rather cold"},
	td{6, "a piercing"},
	td{8, "a chilly"},
	td{11, "a cool"},
	td{15, "a moderate"},
	td{18, "a reasonably warm"},
	td{21, "a hot"},
	td{31, "a scorching"},
	td{36, "a sweltering"},
}

var lastRainTotal, lastOutsideTemp, lastOutsideHumd, avgWind float64

func tweet(message string, subtopic string, interval int64) {
	log.Println("Sending tweet", message)
	services.SendAlert(message, "twitter", subtopic, interval)
}

func checkEvent(ev *pubsub.Event) {
	device := services.Config.LookupDeviceName(ev)
	switch device {
	case services.Config.Weather.Outside.Rain:
		rain := ev.Fields["all_total"].(float64)
		if lastRainTotal != 0.0 && rain > lastRainTotal {
			dayTotal := ev.Fields["day_total"]
			message := fmt.Sprintf("It's raining! (%.2fmm today)", dayTotal)
			tweet(message, "rain", 7200)
		}
		lastRainTotal = rain
	case services.Config.Weather.Outside.Temp:
		temp := ev.Fields["temp"].(float64)
		if lastOutsideTemp != 0.0 && lastOutsideTemp >= 0 && temp < 0 {
			tweet("Brrr, it's gone below zero!", "temp", 7200)
		}
		lastOutsideTemp = temp

		humd, ok := ev.Fields["humidity"].(float64)
		if ok && lastOutsideHumd != 0.0 && lastOutsideHumd < 96 && humd >= 96 {
			tweet("Looks like rain...", "humidity", 7200)
		}
		lastOutsideHumd = humd
	case services.Config.Weather.Outside.Wind:
		speed := ev.Fields["speed"].(float64)
		// about 2 minutes worth moving average
		avgWind = avgWind*39/40 + speed*1/40
		if avgWind > services.Config.Weather.Windy {
			mph := avgWind * 2.237
			msg := fmt.Sprintf("It's windy outside - %.1fmph!", mph)
			tweet(msg, "wind", 7200)
		}
	}
}

// Lookup descriptive text for given temperate range
func getTempDesc(t float64, temps []td) string {
	for _, temp := range temps {
		if t < temp.temp {
			return temp.noun
		}
	}
	return ""
}

// Generate weather message for yesterday
func weatherStats() string {
	highest := getLast24("garden.temp", "max")
	highestDesc := getTempDesc(highest, highTemperatures)
	lowest := getLast24("garden.temp", "min")
	lowestDesc := getTempDesc(lowest, lowTemperatures)
	if lowest == 0 && highest == 0 {
		return "Weather: I didn't get any outside temperature data yesterday!"
	}
	return fmt.Sprintf("Weather: Outside it got up to %s %.1f°C and went down to %s %.1f°C in the last 24 hours.",
		highestDesc, highest,
		lowestDesc, lowest)
}

var gr graphite.IGraphite

// Get last 24 hour temperature min/max
func getLast24(sensor string, cf string) float64 {
	target := fmt.Sprintf(`summarize(sensor.%s.%s,"100y","%s")`, sensor, cf, cf)
	data, err := gr.Query("-24h", "now", target)
	if err != nil {
		log.Println("Failed to get graphite data")
		return 0.0
	}
	return data[0].Datapoints[0].Value
}

func tick() {
	// send weather stats
	msg := weatherStats()
	tweet(msg, "daily", 0)
}

// Service weather
type Service struct{}

// ID of the service
func (service *Service) ID() string {
	return "weather"
}

// Run the service
func (service *Service) Run() error {
	gr = graphite.New(services.Config.Graphite.Host)
	// schedule at 08:00
	offset, _ := time.ParseDuration("8h")
	repeat, _ := time.ParseDuration("24h")
	ticker := util.NewScheduler(offset, repeat)
	events := services.Subscriber.FilteredChannel("rain", "temp", "wind")
	for {
		select {
		case ev := <-events:
			checkEvent(ev)
		case <-ticker.C:
			tick()
		}
	}
}
