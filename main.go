package main

import (
	"fmt"
	"os"
	"strconv"

	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gorilla/websocket"
)

var (
	serverURL            string
	numClients           int
	messagesPerSecond    float64
	loglevel             string
	maxReconnectAttempts int
	reconnectDelay       time.Duration
)

const min int = 10000000
const max int = 99999999

func lookupEnvString(env string, defaultParam string) string {
	param, found := os.LookupEnv(env)
	if !found {
		param = defaultParam
	}
	return param
}

func lookupEnvint64(env string, defaultParam int64) int64 {
	param, found := os.LookupEnv(env)
	if !found {
		param = strconv.FormatInt(int64(defaultParam), 10)
	}
	i, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		i = 1
	}
	return i
}

func lookupEnvFloat64(env string, defaultParam float64) float64 {
	param, found := os.LookupEnv(env)
	if !found {
		param = fmt.Sprintf("%f", defaultParam)
	}
	i, err := strconv.ParseFloat(param, 64)
	if err != nil {
		i = 1
	}
	return i
}

func init() {
	// CLI flags for the application
	// # define Environment Variable

	server := lookupEnvString("WSC_SERVER", "127.0.0.1")
	sport := lookupEnvString("WSC_PORT", "8990")
	serverURL = server + ":" + sport

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
			logger.Errorf("Client %d connected to server %s", clientID, serverURL)
			break
		}
	}
	return conn, nil
}

// Client function for each WebSocket client
func startClient(clientID int, wg *sync.WaitGroup, logger *zap.SugaredLogger) {
	defer wg.Done()

	rand.New(rand.NewSource(int64(clientID))) // maintain the same client id and it will be the same between stoping and stating app
	id := rand.Intn(max-min+1) + min
	// Connect to the WebSocket server
	addr := fmt.Sprintf("%s/ws/%d", serverURL, id)
	conn, err := connectToServer(addr, clientID, logger)
	// conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		logger.Errorf("Client %d: Error connecting to server: %v", clientID, err)
		return
	}

	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

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
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
					logger.Infof("Client %d: Connection closed by server, attempting to reconnect...", clientID)
					conn, err = connectToServer(addr, clientID, logger)
					//maybe to quit if error now
					break
				}
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
