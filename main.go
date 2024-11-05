package main

import (
	"flag"
	"fmt"

	//"log"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gorilla/websocket"
)

var (
	serverURL         string
	numClients        int
	messagesPerSecond float64
	loglevel          string
)

const min int = 10000000
const max int = 99999999

func init() {
	// CLI flags for the application
	flag.StringVar(&serverURL, "url", "ws://localhost:8080", "WebSocket server URL")
	flag.IntVar(&numClients, "clients", 1, "Number of WebSocket clients")
	flag.Float64Var(&messagesPerSecond, "rate", 1.0, "Rate of messages per second")
	flag.StringVar(&loglevel, "Loglevl", "debug", "Loglevel can be  [error, warning, info, debug] defasult is debug")
	flag.Parse()
}

func getLevelLogger(loglevel string) zapcore.Level {
	switch {
	case loglevel == "debug":
		return zap.DebugLevel
	case loglevel == "info":
		return zap.InfoLevel
	case loglevel == "warning":
		return zap.WarnLevel
	case loglevel == "errpr":
		return zap.ErrorLevel
	default:
		return zap.DebugLevel
	}
}

// Client function for each WebSocket client
func startClient(clientID int, wg *sync.WaitGroup, logger *zap.SugaredLogger) {
	defer wg.Done()

	rand.New(rand.NewSource(int64(clientID))) // maintain the same client id and it will be the same between stoping and stating app
	id := rand.Intn(max-min+1) + min
	// Connect to the WebSocket server
	addr := fmt.Sprintf("%s/ws/%d", serverURL, id)
	conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		logger.Errorf("Client %d: Error connecting to server: %v", clientID, err)
		return
	}
	defer conn.Close()

	logger.Infof("Client %d connected to server %s", clientID, serverURL)

	// Send messages at the defined rate
	ticker := time.NewTicker(time.Second / time.Duration(messagesPerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send a message
			t := time.Now()

			message := fmt.Sprintf("%d:%d.%d", id, t.Unix(), t.Nanosecond())
			if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
				logger.Errorf("Client %d: Error sending message: %v", clientID, err)
				return
			}
			logger.Infof("Client %d: Message sent: %s", clientID, message)

		default:
			// Read messages from the server
			_, msg, err := conn.ReadMessage()
			if err != nil {
				logger.Errorf("Client %d: Error reading message: %v", clientID, err)
				return
			}

			// Handle incoming message
			logger.Infof("Client %d vin %d: Received message: %s", clientID, id, string(msg))
			// Handle ping from server
			if string(msg) == "ping" {
				logger.Debugf("Client %d: Responding to ping", clientID)
				err := conn.WriteMessage(websocket.PongMessage, []byte("pong"))
				if err != nil {
					logger.Errorf("Client %d vin %d: Error responding to ping: %v", clientID, id, err)
					return
				}
			}

			// Respond to requests from the server with a message containing the timestamp
			if len(msg) > 0 {
				t := time.Now()
				timestampMessage := fmt.Sprintf("%d|%d.%d|%s", id, t.Unix(), t.Nanosecond(), string(msg))
				err := conn.WriteMessage(websocket.TextMessage, []byte(timestampMessage))
				if err != nil {
					logger.Errorf("Client %d vin %d: Error sending timestamp message: %v", clientID, id, err)
					return
				}
				logger.Infof("Client %d: Sent timestamp message: %s", clientID, timestampMessage)
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

	Logger.Infof("Starting WebSocket Client: Server URL=%s, Clients=%d, Rate=%.2f messages/sec", serverURL, numClients, messagesPerSecond)

	var wg sync.WaitGroup

	// Start multiple WebSocket clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go startClient(i+1, &wg, Logger)
	}

	// Wait for all clients to finish
	wg.Wait()
}
