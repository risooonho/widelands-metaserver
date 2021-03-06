package relayinterface

import (
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"
)

// ClientRPC is an internal struct which implements relayinterface.Client
// over a RPC connection.
type ClientRPC struct {
	callback ClientCallback
	relay    *rpc.Client
	listener net.Listener
}

// ClientRPCMethods is a helper struct so only some methods are exposed to RPC.
type ClientRPCMethods struct {
	client *ClientRPC
}

// NewClientRPC creates a struct that implements relayinterface.Client over RPC.
// An RPC server running on localhost:7398 is assumed.
// Methods of the given callback are called with notifications of the server.
func NewClientRPC(callback ClientCallback) Client {
	client := &ClientRPC{
		callback: callback,
	}

	if !client.connect() {
		return nil
	}

	// Open our rpc server
	rpcLn, err := net.Listen("tcp", ":7399")
	if err != nil {
		log.Printf("Error when listening for RPC calls: %v", err)
		return nil
	}
	client.listener = rpcLn

	// Run our rpc server
	clientMethods := &ClientRPCMethods{
		client: client,
	}

	rpc.Register(clientMethods)

	go func() {
		for {
			conn, err := rpcLn.Accept()
			if err != nil {
				continue
			}
			go jsonrpc.ServeConn(conn)
		}
	}()

	return client
}

// Open connection to relay server
func (client *ClientRPC) connect() bool {
	connection, err := net.DialTimeout("tcp", "localhost:7398", time.Duration(10)*time.Second)
	if err != nil {
		log.Printf("Unable to connect to relay server at localhost: %v", err)
		return false
	}
	client.relay = jsonrpc.NewClient(connection)
	log.Println("Connected to relay server")
	return true
}

// CloseConnection terminates the connection to the relay server.
func (client *ClientRPC) CloseConnection() {
	client.listener.Close()
}

// CreateGame tells the relay server to start a game with the given name.
// The host position in the game is protected by the given password
func (client *ClientRPC) CreateGame(name string, hostPassword string) bool {
	// Tell relay to host game
	success := false
	data := GameData{
		Name:     name,
		Password: hostPassword,
	}
	for i := 0; i < 2; i++ {
		err := client.relay.Call("ServerRPCMethods.NewGame", data, &success)
		if err == nil {
			break
		}
		if err == rpc.ErrShutdown {
			if !client.connect() {
				log.Printf("ClientRPC: Lost connection to relay and are unable to reconnect")
				return false
			}
			log.Printf("ClientRPC: Lost connection to relay but was able to reconnect")
		} else {
			log.Printf("ClientRPC  error: %v", err)
			return false
		}
	}
	return success
}

func (client *ClientRPC) RemoveGame(name string) bool {
	// Tell relay to remove game
	success := false
	data := GameData{
		Name:     name,
		Password: "",
	}
	for i := 0; i < 2; i++ {
		err := client.relay.Call("ServerRPCMethods.RemoveGame", data, &success)
		if err == nil {
			break
		}
		if err == rpc.ErrShutdown {
			if !client.connect() {
				log.Printf("ClientRPC: Lost connection to relay and are unable to reconnect")
				return false
			}
			log.Printf("ClientRPC: Lost connection to relay but was able to reconnect")
		} else {
			log.Printf("ClientRPC  error: %v", err)
			return false
		}
	}
	return success
}

// GameConnected is called by the relay over rpc when a host connected to a game.
func (client *ClientRPCMethods) GameConnected(in *GameData, response *bool) (err error) {
	client.client.callback.GameConnected(in.Name)
	return nil
}

// GameClosed is called by the relay over rpc when a game has ended.
func (client *ClientRPCMethods) GameClosed(in *GameData, response *bool) (err error) {
	client.client.callback.GameClosed(in.Name)
	return nil
}

// GameClosed is called by the relay over rpc when a game has ended.
func (client *ClientRPCMethods) Status(in *string, response *ServerStatus) (err error) {
	*response = *client.client.callback.Status()
	return nil
}
