// Package gfsmux is a multiplexing library for Golang.
//
// It relies on an underlying connection to provide reliability and ordering,
// such as GFCP, and provides stream-oriented multiplexing over a single channel.
package gfsmux // import "go.gridfinity.dev/gfsmux"

import (
	"errors"
	"fmt"
	"io"
	"math"
	"time"
)

// Config is used to tune the Smux session
type Config struct {
	// SMUX Protocol version, support 1,2
	Version int

	// Disabled keepalive
	KeepAliveDisabled bool

	// KeepAliveInterval is how often to send a NOP command to the remote
	KeepAliveInterval time.Duration

	// KeepAliveTimeout is how long the session
	// will be closed if no data has arrived
	KeepAliveTimeout time.Duration

	// MaxFrameSize is used to control the maximum
	// frame size to sent to the remote
	MaxFrameSize int

	// MaxReceiveBuffer is used to control the maximum
	// number of data in the buffer pool
	MaxReceiveBuffer int

	// MaxStreamBuffer is used to control the maximum
	// number of data per stream
	MaxStreamBuffer int
}

// DefaultConfig is used to return a default Configuration
func DefaultConfig() *Config {
	return &Config{
		Version:           1,
		KeepAliveInterval: 10 * time.Second,
		KeepAliveTimeout:  30 * time.Second,
		MaxFrameSize:      32768,
		MaxReceiveBuffer:  4194304,
		MaxStreamBuffer:   65536,
	}
}

// VerifyConfig is used to verify the sanity of Configuration
func VerifyConfig(
	Config *Config,
) error {
	if !(Config.Version == 1 || Config.Version == 2) {
		return errors.New(
			"unsupported protocol version",
		)
	}
	if !Config.KeepAliveDisabled {
		if Config.KeepAliveInterval == 0 {
			return errors.New(
				"keep-alive interval must be positive",
			)
		}
		if Config.KeepAliveTimeout < Config.KeepAliveInterval {
			return fmt.Errorf(
				"keep-alive timeout must be larger than keep-alive interval",
			)
		}
	}
	if Config.MaxFrameSize <= 0 {
		return errors.New(
			"max frame size must be positive",
		)
	}
	if Config.MaxFrameSize > 65535 {
		return errors.New(
			"max frame size must not be larger than 65535",
		)
	}
	if Config.MaxReceiveBuffer <= 0 {
		return errors.New(
			"max receive buffer must be positive",
		)
	}
	if Config.MaxStreamBuffer <= 0 {
		return errors.New(
			"max stream buffer must be positive",
		)
	}
	if Config.MaxStreamBuffer > Config.MaxReceiveBuffer {
		return errors.New(
			"max stream buffer must not be larger than max receive buffer",
		)
	}
	if Config.MaxStreamBuffer > math.MaxInt32 {
		return errors.New(
			"max stream buffer cannot be larger than 2147483647",
		)
	}
	return nil
}

// Server is used to initialize a new server-side Connection.
func Server(
	Conn io.ReadWriteCloser,
	Config *Config,
) (
	*Session,
	error,
) {
	if Config == nil {
		Config = DefaultConfig()
	}
	if err := VerifyConfig(
		Config,
	); err != nil {
		return nil, err
	}
	return newSession(
		Config,
		Conn,
		false,
	), nil
}

// Client is used to initialize a new client-side Connection.
func Client(
	Conn io.ReadWriteCloser,
	Config *Config,
) (
	*Session,
	error,
) {
	if Config == nil {
		Config = DefaultConfig()
	}

	if err := VerifyConfig(
		Config,
	); err != nil {
		return nil, err
	}
	return newSession(
		Config,
		Conn,
		true,
	), nil
}
