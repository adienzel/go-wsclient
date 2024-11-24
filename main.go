package main

import (
	"fmt"
	// "os"
	// "strconv"

	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gorilla/websocket"
)

var (
	server               string
	port                 uint16
	numClients           int
	numOfPorts           uint16
	messagesPerSecond    float64
	loglevel             string
	maxReconnectAttempts int
	reconnectDelay       time.Duration
)

const min int = 10000000
const max int = 99999999

type State int

const (
	CLOSE_STATE State = iota
	OPEN_STATE
)

type Connections struct {
	id        int
	state     State
	client_id int
	addr      string
	conn      *websocket.Conn
}

func init() {
	// CLI flags for the application
	// # define Environment Variable

	server = lookupEnvString("WSC_SERVER", "127.0.0.1")
	port = uint16(lookupEnvint64("WSC_PORT", 8020))
	numOfPorts = uint16(lookupEnvint64("WSC_NUMBER_OF_PORTS", 100))
	//serverURL = server + ":" + sport

	numClients = int(lookupEnvint64("WSC_NUMBER_OF_CLIENTS", int64(1)))

	// # Float number smaller than 1.0 is smaller than 1per secnd and larger means the number of messages per second
	// # it is calculated as 1/WS_MESSAGES_PER_SECOND for the time delay between messages
	messagesPerSecond = lookupEnvFloat64("WSC_MESSAGES_PER_SECOND", 1.0)

	loglevel = lookupEnvString("WSC_LOG_LEVEL", "debug")

	maxReconnectAttempts = int(lookupEnvint64("WSC_MAX_RECONNECT_ATTEMPT", int64(10)))
	reconnectDelay = time.Duration((lookupEnvint64("WSC_DELAY_BETWEEN_RECONNECT", int64(5))))
}

func getLevelLogger(loglevel string) zapcore.Level {
	switch {
	case loglevel == "debug":
		return zap.DebugLevel
	case loglevel == "info":
		return zap.InfoLevel
	case loglevel == "warning":
		return zap.WarnLevel
	case loglevel == "error":
		return zap.ErrorLevel
	default:
		return zap.DebugLevel
	}
}

func connectToServer(addr string, clientID int, logger *zap.SugaredLogger) (conn *websocket.Conn, err error) {
	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		// Attempt to connect
		conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
		if err != nil {
			logger.Errorf("Client %d: Error connecting to server on attempt %d: %v", clientID, attempt, err)
			if attempt < maxReconnectAttempts {
				logger.Infof("Client %d: Retrying in %s...", clientID, reconnectDelay)
				time.Sleep(reconnectDelay)
			} else {
				logger.Errorf("Client %d: Reached max reconnect attempts, exiting.", clientID)
				return nil, fmt.Errorf("client %d: reached max reconnect attempts, exiting", clientID)
			}
		} else {
			logger.Errorf("Client %d connected to server %s", clientID, addr)
			break
		}
	}
	return conn, nil
}

func cleanConnections(connction_list *[]Connections, logger *zap.SugaredLogger) {
	for _, e := range *connction_list {
		if e.conn != nil {
			err := e.conn.Close()
			if err != nil {
				logger.Errorf("Failed to close connection: %v", err)
			}
		}
	}
	logger.Info("All connections closed.")
}

func (e *Connections) readMessage(logger *zap.SugaredLogger) (message_type int, msg []byte, err error) {
	message_type, msg, err = e.conn.ReadMessage()
	if err != nil {
		logger.Errorf("Client %d: Error reading message: %v", e.client_id, err)
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
			logger.Infof("Client %d: Connection closed by server, attempting to reconnect...", e.client_id)
			e.conn, err = connectToServer(e.addr, e.client_id, logger)
			if err != nil {
				logger.Errorf("Client %d: failed to reconnect: %v", e.client_id, err)
				return message_type, nil, fmt.Errorf("Client %d: failed to reconnect: %v", e.client_id, err)
			}
			logger.Infof("Client %d reconnected ", e.client_id)
			return message_type, nil, fmt.Errorf("Client %d reconnected ", e.client_id) // if there was a message it losted
		}
	}

	// Handle incoming message
	// Handle ping from server
	switch message_type {
	case websocket.PingMessage:
		logger.Debugf("Client %d: Responding to ping", e.client_id)
		err := e.conn.WriteMessage(websocket.PongMessage, []byte("pong"))
		if err != nil {
			return message_type, msg, fmt.Errorf("Client %d vin %d: Error responding to ping: %v", e.client_id, e.id, err)
		}
		return message_type, msg, nil // wwe need to test thhat this is ping to avoid sending something
	case websocket.PongMessage:
		return message_type, msg, nil // wwe need to test thhat this is ping to avoid sending something
	default:
		logger.Debugf("Client %d vin %d: Received message: %s", e.client_id, e.id, string(msg))
		return message_type, msg, fmt.Errorf("Client %d vin %d: Error responding to ping: %v", e.client_id, e.id, err)
		// we need to test thhat this is ping to avoid sending something
	}
}

func (e *Connections) sendMessage(msg []byte, logger *zap.SugaredLogger) (err error) {
	// Respond to requests from the server with a message containing the timestamp
	if len(msg) > 0 {
		t := time.Now()
		timestampMessage := fmt.Sprintf("%d|%d.%d|%s", e.id, t.Unix(), t.Nanosecond(), string(msg))
		err = e.conn.WriteMessage(websocket.TextMessage, []byte(timestampMessage))
		if err != nil {
			logger.Debugf("Client %d vin %d: Error sending timestamp message: %v", e.client_id, e.id, err)
			return fmt.Errorf("Client %d vin %d: Error sending timestamp message: %v", e.client_id, e.id, err)
		}
		logger.Debugf("Client %d: Sent timestamp message: %s", e.client_id, timestampMessage)
	}
	return nil
}

func createConnections(connction_list *[]Connections, server_port uint16, startClientID int, numberOfClients int, logger *zap.SugaredLogger) bool {
	rand.New(rand.NewSource(int64(startClientID))) // maintain the same client id and it will be the same between stoping and stating app
	//TODO define list to hold connections
	for i := startClientID; i < startClientID+numberOfClients; i++ {
		var conn_entry Connections
		id := rand.Intn(max-min+1) + min
		// Connect to the WebSocket server
		addr := fmt.Sprintf("%s:%d/ws/%d", server, server_port, id)
		conn, err := connectToServer(addr, startClientID+i, logger)
		// conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
		if err != nil {
			logger.Errorf("Client %d: Error connecting to server: %v", startClientID+i, err)
			defer cleanConnections(connction_list, logger)
			return false
		}

		conn_entry.id = id
		conn_entry.state = OPEN_STATE
		conn_entry.client_id = startClientID + i
		conn_entry.addr = addr
		conn_entry.conn = conn

		*connction_list = append(*connction_list, conn_entry)
		logger.Infof("Client %d connected to server %s:%d", startClientID+i, server, port+uint16(i))

	}
	return true
}

// Client function for each WebSocket client
func startClient(server_port uint16, startClientID int, numberOfClients int, wg *sync.WaitGroup, logger *zap.SugaredLogger) {
	defer wg.Done()

	var connction_list []Connections
	defer cleanConnections(&connction_list, logger)

	if !createConnections(&connction_list, server_port, startClientID, numberOfClients, logger) {
		return
	}

	// Send messages at the defined rate
	ticker := time.NewTicker(time.Second / time.Duration(messagesPerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send a message
			for _, e := range connction_list {
				t := time.Now()
				message := fmt.Sprintf("%d:%d.%d", e.id, t.Unix(), t.Nanosecond())
				if err := e.conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
					logger.Errorf("Client %d: Error sending message: %v", e.client_id, err)
					return
				}
				logger.Debugf("Client %d: Message sent: %s", e.client_id, message)
			}
		default:
			// Read messages from the server
			for _, e := range connction_list {
				message_type, msg, err := e.readMessage(logger)
				if err != nil {
					if message_type == websocket.PingMessage || message_type == websocket.PongMessage {
						continue
					} else {
						logger.Errorf("got error reading message %v", err)
						//TODO  add counter on error messages
						continue
					}
				}

				if len(msg) > 0 {
					// will need to check if responsae or RPC request
					if message_type == websocket.TextMessage {
						err = e.sendMessage(msg, logger)
					}
				}
			}
		}
	}
}

func main() {

	var Logger *zap.SugaredLogger
	var err error
	level := zap.NewAtomicLevelAt(getLevelLogger(loglevel))
	encoder := zap.NewProductionEncoderConfig()

	zapConfig := zap.NewProductionConfig()
	zapConfig.EncoderConfig = encoder
	zapConfig.Level = level
	// zapConfig.Development = config.IS_DEVELOP_MODE
	zapConfig.Encoding = "json"
	//zapConfig.InitialFields = map[string]interface{}{"idtx": "999"}
	zapConfig.OutputPaths = []string{"stdout"} // can add later a log file
	zapConfig.ErrorOutputPaths = []string{"stderr"}
	logger, err := zapConfig.Build()

	if err != nil {
		panic(err)
	}
	Logger = logger.Sugar()

	if numOfPorts == 0 {
		panic(fmt.Errorf("Number of ports is 0"))
	}

	if numOfPorts > uint16(numClients) {
		numClients = int(numOfPorts)
	}
	Logger.Infof("Starting WebSocket Client: Server URL=%s, Clients=%d, Rate=%.2f messages/sec",
		server, numClients, messagesPerSecond)

	var wg sync.WaitGroup

	// Start multiple WebSocket clients

	for i := 0; i < int(numOfPorts); i++ {
		wg.Add(1)
		go startClient(port+uint16(i), i, numClients/int(numOfPorts), &wg, Logger)
	}

	// Wait for all clients to finish
	wg.Wait()
}
