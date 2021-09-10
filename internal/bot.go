package internal

import (
	"gobot.io/x/gobot"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/platforms/mqtt"
	"log"
)

type PirSensor interface {
	gobot.Eventer
	Start() (err error)
	Halt() (err error)
	Name() string
	SetName(n string)
	Pin() string
	Connection() gobot.Connection
}

type MotionDetection struct {
	Driver      *gpio.PIRMotionDriver
	Adaptor     gobot.Connection
	MqttAdaptor *mqtt.Adaptor

	Config Config
}

func (m *MotionDetection) publishMessage(msg []byte) {
	success := m.MqttAdaptor.Publish(m.Config.Topic, msg)
	if success {
		metricsMessagesPublished.WithLabelValues(m.Config.Location).Inc()
	} else {
		metricsMessagePublishErrors.WithLabelValues(m.Config.Location).Inc()
	}
}

func AssembleBot(motion *MotionDetection) *gobot.Robot {
	errorCnt := 0
	work := func() {
		motion.Driver.On(gpio.MotionDetected, func(data interface{}) {
			metricsMotionsDetected.WithLabelValues(motion.Config.Location).Inc()
			metricsMotionTimestamp.WithLabelValues(motion.Config.Location).SetToCurrentTime()
			motion.publishMessage([]byte("ON"))
			if motion.Config.LogMotions {
				log.Println("Detected motion")
			}
		})

		motion.Driver.On(gpio.MotionStopped, func(data interface{}) {
			motion.publishMessage([]byte("OFF"))
			if motion.Config.LogMotions {
				log.Println("Motion stopped")
			}
		})

		motion.Driver.On(gpio.Error, func(data interface{}) {
			if errorCnt > 10 {
				log.Fatalf("Too many errors, shutting down")
			}
			errorCnt += 1
			log.Printf("GPIO error: %v", data)
		})
	}

	adaptors := []gobot.Connection{motion.Adaptor}
	if motion.MqttAdaptor != nil {
		adaptors = append(adaptors, motion.MqttAdaptor)
	}

	return gobot.NewRobot(BotName,
		adaptors,
		[]gobot.Device{motion.Driver},
		work,
	)
}
