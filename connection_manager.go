package glink

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"math"
	"sync"
	"time"
)

const (
	minConnectionAge = 15 * time.Second
)

// ConnectionManager is the structure responsible for managing gRPC connections
type ConnectionManager struct {
	mu               sync.Mutex
	connection1      *Connection
	maxConnectionAge time.Duration
	maxRetries       int
	serviceAddress   string
	dialOpts         []grpc.DialOption
}

// New creates a new ConnectionManager instance for the specified service address
// and establishes the necessary configuration for managing gRPC connections.
//
// Parameters:
//   - serviceAddress (string): The address of the service that the connection manager
//     will manage connections to (e.g., "localhost:50051").
//   - maxConnectionAge (time.Duration): The maximum duration for which a connection can
//     be considered valid before it needs to be re-established (e.g., 5 * time.Minute).
//   - maxRetries (uint): The maximum number of retry attempts allowed in case of failure
//     when trying to establish a connection or reconnect.
//   - logger (bool): A flag that determines whether logging is enabled for this connection manager.
//     If set to true, logging will be enabled (e.g., to track connection status, retries, etc.).
func New(serviceAddress string, maxConnectionAge time.Duration, maxRetries uint, logger bool) *ConnectionManager {

	// Enable log
	if logger == true {
		GetLogger().Enable()
	}

	// Ensure min connection time
	if maxConnectionAge < minConnectionAge {
		maxConnectionAge = minConnectionAge
	}

	cm := &ConnectionManager{
		serviceAddress:   serviceAddress,
		maxConnectionAge: maxConnectionAge,
		maxRetries:       int(maxRetries),
		connection1:      NewConnection(serviceAddress),
	}

	dialOptions := []grpc.DialOption{}
	dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	dialOptions = append(dialOptions, grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`))
	dialOptions = append(dialOptions, grpc.WithUnaryInterceptor(retryInterceptor(cm)))

	cm.dialOpts = dialOptions

	return cm
}

// connect establishes a new connection and assigns it to the connection manager
func (cm *ConnectionManager) connect() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, grpcErr := grpc.NewClient(
		cm.serviceAddress,
		cm.dialOpts...,
	)

	if grpcErr != nil {
		return grpcErr
	}

	cm.connection1.SetConnection(conn, cm.maxConnectionAge)

	return nil
}

func (cm *ConnectionManager) Close() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.connection1.Close()
}

func (cm *ConnectionManager) GetConnection() (*grpc.ClientConn, error) {
	var err error

	// First time connection will look as expired
	if cm.connection1.IsExpired() {
		err = cm.connect()
		return cm.connection1.Connection(), err
	}

	return cm.connection1.Connection(), err
}

func (cm *ConnectionManager) TryReconnect() (*grpc.ClientConn, error) {
	var err error

	if time.Now().Sub(cm.connection1.lastConnection) >= minConnectionAge {
		err = cm.connect()
		if err != nil {
			return cm.connection1.Connection(), err
		}
	}

	return cm.connection1.Connection(), err
}

func (cm *ConnectionManager) ShouldReconnect() bool {
	return cm.connection1.IsExpired()
}

func (cm *ConnectionManager) backoffDuration(attempt int) time.Duration {
	var baseRetryDelay = 100 * time.Millisecond

	delay := float64(baseRetryDelay) * math.Pow(2, float64(attempt))

	return time.Duration(delay)
}
