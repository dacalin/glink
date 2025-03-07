# glink: gRPC Connection Manager

When using gRPC for communication between services, traditional load balancers (such as those that rely on HTTP/1.x) 
often do not work well with gRPC, primarily due to the protocol's reliance on HTTP/2. HTTP/2, which is the 
underlying protocol for gRPC, introduces several features that are beneficial for performance
(such as multiplexing multiple requests over a single connection), but these features can interfere with the way 
traditional load balancers manage traffic.

Why Traditional Load Balancers Don’t Work Well with gRPC:
1. Persistent Connections: Unlike HTTP/1.x, HTTP/2 allows multiple requests and responses to be multiplexed over a 
single connection. This means that multiple streams can be handled over one connection, making it difficult for 
traditional load balancers to efficiently distribute traffic based on connection-specific metrics like IP or port.

2. Stream-Based Routing: HTTP/2 makes heavy use of streams within a single connection, and many load balancers are not
designed to understand or route traffic on a per-stream basis. As a result, they might route all traffic from a single 
connection to the same backend server, leading to inefficient load distribution.

3. Connection Reuse: gRPC connections tend to be reused for multiple requests. If the connection is routed to a 
particular server initially, subsequent requests over the same connection will be sent to that server, which might
not result in optimal load balancing.

4. gRPC Load Balancing: gRPC includes built-in load balancing mechanisms that are more suited to handle the unique
needs of HTTP/2 connections. The typical approach is using the round_robin load balancing strategy, 
which is a built-in strategy for distributing connections across servers, but this strategy requires 
specialized handling and configuration that is not available in traditional HTTP/1.x load balancers.

`glink` is designed to solve this problem by providing a connection manager that handles gRPC-specific load balancing 
strategies (like round_robin) and ensures that gRPC connections are properly managed, even as servers are added or 
removed. By using this package, you can ensure that gRPC client connections are handled efficiently, with retries and
reconnections, while respecting the nuances of HTTP/2 and gRPC's connection management model.

## Features

- **Connection Management**: Handles gRPC connection lifecycle, including expiration and reconnection. It add an interceptor to reconnect in case of a connection error. Will generate new connections at specified interval. This force to get an updated list of instances for proper load balancing.
- **Exponential Backoff**: Implements exponential backoff for retries.
- **Round Robin Load Balancing**: Uses round robin for load balancing when connecting to multiple instances of a service.
- **Logging Support**: Supports enabling or disabling logging to track connection status and retries.

## Installation

To install `glink`, use `go get` to retrieve the package into your Go workspace.

```bash
go get github.com/dacalin/glink
```
## Usage

Here’s a simple example demonstrating how to use the ConnectionManager to manage gRPC connections.

```go
package main

import (
	"fmt"
	"log"
	"time"
	"github.com/dacalin/glink" // Import the glink package
	"google.golang.org/grpc"
)

func main() {
	// Service address for your gRPC server
	serviceAddress := "localhost:50051"
	
	// Create a new connection manager. Adjust the time as need.
	// If you set a long time and your service scaled up,  your client might not use
	// all available instances, but will get a new connection if it fails usign
	// an interceptor
	// If the time is too low, you will generate connections very frequently with 
	// could not be optimal.
	cm := glink.New(serviceAddress, 5*time.Minute, 3, true)

	// Get a connection from the ConnectionManager. The connection will be updated 
	// automatically after 5*time.Minute, or if connection throws DeadlineExceeded 
	// or Unavailable.
	conn, err := cm.GetConnection()
	if err != nil {
		log.Fatalf("Error establishing connection: %v", err)
	}
	defer cm.Close()

	// Build a new object pb.NewProtocolClient in each request with conn object.
	// This will add near zero overhead, but will guaranteed you connection is in shape.
	yourclient := pb.NewProtocolClient(conn)
	yourclient.GetData1()
	yourclient.GetData2()

}

```

##  Contributing
Contributions to glink are welcome! If you have bug fixes or improvements, feel free to submit a pull request.


