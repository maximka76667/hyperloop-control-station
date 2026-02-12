package tcp

import (
	"fmt"
	"net"
	"time"

	"github.com/rs/zerolog"
)

type Client struct {
	config  ClientConfig
	address string

	currentRetries int

	logger zerolog.Logger
}

// NewClient inits a ClientConfig with good defaults and the provided information
func NewClient(address string, config ClientConfig, baseLogger zerolog.Logger) Client {
	logger := baseLogger.With().Str("localAddres", config.LocalAddr.String()).Str("remoteAddress", address).Logger()
	return Client{
		config: config,

		address: address,

		currentRetries: 0,

		logger: logger,
	}
}

// Dial attempts to connect with the client
func (client *Client) Dial() (net.Conn, error) {
	client.logger.Info().Msg("dialing")

	for {
		// Attempt the connection immediately
		conn, err := client.config.DialContext(client.config.Context, "tcp", client.address)

		// Connection successful
		if err == nil {
			client.logger.Info().Msg("connected")
			client.currentRetries = 0
			return conn, nil
		}

		// Check if we should even bother retrying
		if client.config.Context.Err() != nil {
			return nil, client.config.Context.Err()
		}

		client.currentRetries++

		// Check if we hit the limit, maximum connection retries exceeded
		if client.config.MaxConnectionRetries > 0 && client.currentRetries >= client.config.MaxConnectionRetries {
			client.logger.Debug().Int("max", client.config.MaxConnectionRetries).Msg("max connection retries exceeded")
			return nil, ErrTooManyRetries{
				Max:     client.config.MaxConnectionRetries,
				Network: "tcp",
				Remote:  client.address,
			}
		}

		// Backoff and Sleep ONLY AFTER a failure
		backoffDuration := client.config.ConnectionBackoffFunction(client.currentRetries)
		client.logger.Error().Err(err).Dur("backoff", backoffDuration).Int("retry", client.currentRetries).Msg("retrying after backoff")

		time.Sleep(backoffDuration)
	}
}

type ErrTooManyRetries struct {
	Max     int
	Network string
	Remote  string
}

func (err ErrTooManyRetries) Error() string {
	return fmt.Sprintf("tried to reconnect over %d times to %s over %s", err.Max, err.Network, err.Remote)
}
