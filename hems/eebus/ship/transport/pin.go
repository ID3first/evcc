package transport

import (
	"errors"
	"time"

	"github.com/andig/evcc/hems/eebus/ship/message"
)

// read pin requirements
func (c *Transport) readPinState() (message.ConnectionPinState, error) {
	timer := time.NewTimer(CmiReadWriteTimeout)
	msg, err := c.ReadMessage(timer.C)

	switch typed := msg.(type) {
	case message.ConnectionPinState:
		return typed, err

	default:
		if err == nil {
			err = errors.New("pin: invalid type")
		}

		return message.ConnectionPinState{}, err
	}
}

const (
	pinReceived = 1 << iota
	pinSent

	pinCompleted = pinReceived | pinSent
)

func (c *Transport) PinState(local, remote string) error {
	pinState := message.ConnectionPinState{
		PinState: message.PinStateNone,
	}

	var status int
	if local != "" {
		pinState.PinState = message.PinStateRequired
		pinState.InputPermission = message.PinInputPermissionOk
	} else {
		// always received if not necessary
		status |= pinReceived
	}

	err := c.WriteJSON(message.CmiTypeControl, message.CmiConnectionPinState{
		ConnectionPinState: pinState,
	})

	timer := time.NewTimer(10 * time.Second)
	for err == nil && status != pinCompleted {
		var msg interface{}
		msg, err = c.ReadMessage(timer.C)
		if err != nil {
			break
		}

		switch typed := msg.(type) {
		// local pin
		case message.ConnectionPinInput:
			// signal error to client
			if typed.Pin != local {
				err = c.WriteJSON(message.CmiTypeControl, message.CmiConnectionPinError{
					message.ConnectionPinError{Error: 1},
				})
			}

			status |= pinReceived

		// remote pin
		case message.ConnectionPinState:
			if typed.PinState == message.PinStateOptional || typed.PinState == message.PinStateRequired {
				if remote != "" {
					err = c.WriteJSON(message.CmiTypeControl, message.CmiConnectionPinInput{
						message.ConnectionPinInput{Pin: remote},
					})
				} else {
					err = errors.New("pin: remote pin required")
				}
			}

			status |= pinSent

		case message.ConnectionPinError:
			err = errors.New("pin: remote pin mismatched")

		case message.ConnectionClose:
			err = errors.New("pin: remote closed")

		default:
			err = errors.New("pin: invalid type")
		}
	}

	return err
}