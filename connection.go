package glink

import (
	"google.golang.org/grpc"
	"sync"
	"time"
)

const (
	gracePeriod = 10 * time.Second
)

type Connection struct {
	id             string
	mu             sync.RWMutex
	connection     *grpc.ClientConn
	lastConnection time.Time
	maxDuration    time.Duration
}

func NewConnection(id string) *Connection {

	return &Connection{id: id, connection: nil}
}

func (c *Connection) Connection() *grpc.ClientConn {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.connection
}

func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	GetLogger().Println("gRPC close connection Id:", c.id)
	c.connection.Close()
	c.connection = nil
}

func (c *Connection) SetConnection(conn *grpc.ClientConn, maxDuration time.Duration) {
	GetLogger().Printf("Settting new connection (ID: %s)", c.id)

	// Swap the connection safely
	c.mu.Lock()

	oldConn := c.connection // Store old connection
	c.connection = conn     // Assign new connection
	c.lastConnection = time.Now()
	c.maxDuration = maxDuration

	c.mu.Unlock()

	// Close the old connection **after unlocking** to avoid blocking
	// Close the old connection after the grace period has elapsed
	if oldConn != nil {
		go func() {
			// Wait for the grace period to elapse before closing the old connection
			time.Sleep(gracePeriod)
			_ = oldConn.Close()
			GetLogger().Printf("Old gRPC connection (ID: %s) closed after grace period.", c.id)
		}()
	}
}

func (c *Connection) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return time.Now().Sub(c.lastConnection) >= c.maxDuration
}
