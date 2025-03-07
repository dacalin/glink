package glink

import (
	"context"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// newBufConn returns a dialer option using a bufconn listener.
func newBufConn() (grpc.DialOption, *bufconn.Listener) {
	lis := bufconn.Listen(bufSize)
	// Create a dummy gRPC server.
	s := grpc.NewServer()
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	dialerOpt := grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	})
	return dialerOpt, lis
}

// TestSetConnectionGracePeriod verifies that when SetConnection is called
// with a new connection, the old connection is closed after the grace period.
func TestSetConnectionGracePeriod(t *testing.T) {
	// Prepare bufconn dial option.
	dialOpt, _ := newBufConn()

	opts := []grpc.DialOption{
		dialOpt,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`),
	}
	conn1, err := grpc.NewClient("bufnet", opts...)

	if err != nil {
		t.Fatalf("Failed to dial bufnet (conn1): %v", err)
	}

	// Create a new Connection instance.
	connObj := NewConnection("test")
	maxDur := 10 * time.Second
	connObj.maxDuration = maxDur

	// Set the first connection.
	connObj.SetConnection(conn1, maxDur)
	oldConn := conn1

	// Wait briefly.
	time.Sleep(200 * time.Millisecond)

	// Create second connection.
	conn2, err := grpc.NewClient("bufnet", opts...)
	if err != nil {
		t.Fatalf("Failed to create second connection: %v", err)
	}
	defer conn2.Close()

	// Set the new connection.
	connObj.SetConnection(conn2, maxDur)

	// Wait for grace period plus a little slack.
	time.Sleep(gracePeriod + 1*time.Second)

	// Check the state of the old connection.
	state := oldConn.GetState()
	if state != connectivity.Shutdown {
		t.Errorf("Expected old connection state to be Shutdown, got: %v", state)
	}
}

// TestIsExpired verifies that IsExpired returns true if the connection is too old.
func TestIsExpired(t *testing.T) {
	// Create a new Connection instance.
	connObj := NewConnection("test")
	// Set maxDuration to 1 second.
	connObj.maxDuration = 1 * time.Second
	connObj.lastConnection = time.Now().Add(-2 * time.Second) // simulate an old connection

	if !connObj.IsExpired() {
		t.Errorf("Expected connection to be expired")
	}
}

// TestConnectionManager_GetConnection verifies that the ConnectionManager correctly manages connections.
func TestConnectionManager_GetConnection(t *testing.T) {
	// Prepare bufconn dial option.
	dialOpt, lis := newBufConn()

	// Create a ConnectionManager instance
	cm := New("bufnet", 5*time.Second, 3, true)
	cm.dialOpts = append(cm.dialOpts, dialOpt)

	// Get a connection from the manager.
	conn, err := cm.GetConnection()
	if err != nil {
		t.Fatalf("Failed to get connection from ConnectionManager: %v", err)
	}

	// Check that the connection is indeed Ready.
	if state := conn.GetState(); state != connectivity.Idle && state != connectivity.Ready {
		t.Fatalf("Expected connection to be Idle or Ready, got: %v", state)
	}

	// Close the connection.
	cm.Close()

	if state := conn.GetState(); state != connectivity.Shutdown {
		t.Fatalf("Expected connection to be Shutdown, got: %v", state)
	}

	lis.Close()
}

// TestConnectionManager_ShouldReconnect verifies that ShouldReconnect returns the correct value.
func TestConnectionManager_ShouldReconnect(t *testing.T) {
	// Prepare bufconn dial option.
	dialOpt, _ := newBufConn()

	// Create a ConnectionManager instance
	cm := New("bufnet", 5*time.Second, 3, true)
	cm.dialOpts = append(cm.dialOpts, dialOpt)

	// Simulate the connection being expired
	cm.connection1.lastConnection = time.Now().Add(-10 * time.Second)

	// Check ShouldReconnect
	if !cm.ShouldReconnect() {
		t.Fatalf("Expected ShouldReconnect to return true, but it returned false")
	}
}

// TestConnectionManager_TryReconnect verifies that TryReconnect attempts a reconnection when needed.
func TestConnectionManager_TryReconnect(t *testing.T) {
	// Prepare bufconn dial option.
	dialOpt, _ := newBufConn()

	// Create a ConnectionManager instance
	cm := New("bufnet", 5*time.Second, 3, true)
	cm.dialOpts = append(cm.dialOpts, dialOpt)

	// Try reconnecting
	conn1, err := cm.TryReconnect()
	if err != nil {
		t.Fatalf("Failed to reconnect: %v", err)
	}
	conn2, err := cm.TryReconnect()
	if err != nil {
		t.Fatalf("Failed to reconnect: %v", err)
	}
	if conn1 != conn2 {
		t.Fatalf("Expected conn1 to be equal to conn2")
	}

	time.Sleep(minConnectionAge + 1*time.Second) // Wait to be able to generate a new connection
	// After 6 seconds connection should be different
	conn3, err := cm.TryReconnect()
	if err != nil {
		t.Fatalf("Failed to reconnect: %v", err)
	}
	if conn1 == conn3 {
		t.Fatalf("Expected conn1 to be different to conn3")
	}

	// Check that the connection is indeed Ready.
	if state := conn3.GetState(); state != connectivity.Idle && state != connectivity.Ready {
		t.Fatalf("Expected connection to be Idle or Ready, got: %v", state)
	}

	// After graceful period conn1 should be closed
	time.Sleep(gracePeriod + 1*time.Second)
	if state := conn1.GetState(); state != connectivity.Shutdown {
		t.Fatalf("Expected conn1 to be Shutdown, got: %v", state)
	}

	// Close the connection.
	cm.Close()

	if state := conn3.GetState(); state != connectivity.Shutdown {
		t.Fatalf("Expected connection to be Shutdown, got: %v", state)
	}
}
