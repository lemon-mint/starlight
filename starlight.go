package starlight

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/lemon-mint/starlight/internal/randpool"
)

const PROTOCOL_VERSION = 1

type StarlightConfig struct {
	AllowPoll      bool // poll01
	AllowLongPoll  bool // lpll01
	AllowSSE       bool // hsse01
	AllowWebsocket bool // webs01

	SessionTimeout       time.Duration
	LongPollTimeout      time.Duration
	HTTPStreamingTimeout time.Duration

	BasePath        string
	DefaultProtocol Protocol

	NotFoundHandler func(w http.ResponseWriter, r *http.Request)
}

func defaultConfig() *StarlightConfig {
	return &StarlightConfig{
		AllowPoll:      true,
		AllowLongPoll:  true,
		AllowSSE:       true,
		AllowWebsocket: true,

		SessionTimeout:       time.Second * 15,
		LongPollTimeout:      time.Second * 10,
		HTTPStreamingTimeout: time.Second * 10,
	}
}

type StarlightOption func(*StarlightConfig)

func WithAllowPoll(allow bool) StarlightOption {
	return func(c *StarlightConfig) {
		c.AllowPoll = allow
	}
}

func WithAllowLongPoll(allow bool) StarlightOption {
	return func(c *StarlightConfig) {
		c.AllowLongPoll = allow
	}
}

func WithAllowSSE(allow bool) StarlightOption {
	return func(c *StarlightConfig) {
		c.AllowSSE = allow
	}
}

func WithAllowWebsocket(allow bool) StarlightOption {
	return func(c *StarlightConfig) {
		c.AllowWebsocket = allow
	}
}

func WithDefaultProtocol(protocol Protocol) StarlightOption {
	return func(c *StarlightConfig) {
		c.DefaultProtocol = protocol
	}
}

func WithNotFoundHandler(handler func(w http.ResponseWriter, r *http.Request)) StarlightOption {
	return func(c *StarlightConfig) {
		c.NotFoundHandler = handler
	}
}

func NewStarlight(options ...StarlightOption) *Starlight {
	var s [8]byte
	randpool.CSPRNG_RAND(s[:])

	var c = defaultConfig()
	for _, option := range options {
		option(c)
	}

	g := &Starlight{
		_config:       c,
		_server_token: hex.EncodeToString(s[:]),
		sessions: &starlightSessionPool{
			id_counter: func() uint64 {
				var b [8]byte
				randpool.CSPRNG_RAND(b[:])
				return binary.LittleEndian.Uint64(b[:])
			}(),
		},
	}

	g.buildDirectoryResponse()

	return g
}

type Starlight struct {
	_config *StarlightConfig

	_server_token           string
	_directory_response     []byte
	_directory_response_len string

	sessions *starlightSessionPool
}

var response_404 = []byte("<html>\r\n" +
	"<head><title>404 Not Found</title></head>\r\n" +
	"<body>\r\n" +
	"<center><h1>404 Not Found</h1></center>\r\n" +
	"<hr><center>nginx</center>\r\n" +
	"</body>\r\n" +
	"</html>\r\n")

var response_404_len = strconv.Itoa(len(response_404))

func handleNotFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Server", "nginx")
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", response_404_len)
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusNotFound)

	w.Write(response_404)
}

var NotFoundHandler = http.HandlerFunc(handleNotFound)

//go:generate stringer -type=Protocol -linecomment
type Protocol uint16

const (
	q_key              = "starlight"
	q_key_directory    = "directory"
	q_key_polling      = "c29bb1b250b6d522" // poll01
	q_key_long_polling = "e14abc88cb6c5dcb" // lpll01
	q_key_http_sse     = "f2ace8d571ac98ae" // hsse01
	q_key_websocket    = "ee99a57a33ec9ca2" // webs01

	PROTOCOL_poll01 Protocol = 10 // poll01
	PROTOCOL_lpll01 Protocol = 11 // lpll01
	PROTOCOL_hsse01 Protocol = 12 // hsse01
	PROTOCOL_webs01 Protocol = 13 // webs01
)

func (g *Starlight) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get(q_key) {
	case q_key_directory:
		g.directoryHandler(w, r)
	case q_key_http_sse:
		// s.httpResponseStreamingHandler(w, r)
	case q_key_websocket:
		// s.websocketHandler(w, r)
	case q_key_polling:
		// s.pollingHandler(w, r)
	default:
		if g._config.NotFoundHandler != nil {
			g._config.NotFoundHandler(w, r)
		}
	}
}

type directoryResponse struct {
	ProtocolVersion int                         `json:"version"`
	BasePath        string                      `json:"base_path"`
	ServerToken     string                      `json:"server_token"`
	Protocols       []directoryResponseProtocol `json:"protocols"`
	Preferred       Protocol                    `json:"preferred"`
}

type directoryResponseProtocol struct {
	Protocol Protocol `json:"protocol"`
	Key      string   `json:"key"`
}

func (g *Starlight) buildDirectoryResponse() {
	dr := directoryResponse{
		ProtocolVersion: PROTOCOL_VERSION,
		ServerToken:     g._server_token,
		BasePath:        g._config.BasePath,
		Preferred:       g._config.DefaultProtocol,
	}

	var protocols []directoryResponseProtocol

	switch {
	case g._config.AllowPoll:
		protocols = append(protocols, directoryResponseProtocol{
			Protocol: PROTOCOL_poll01,
			Key:      q_key_polling,
		})
	case g._config.AllowLongPoll:
		protocols = append(protocols, directoryResponseProtocol{
			Protocol: PROTOCOL_lpll01,
			Key:      q_key_long_polling,
		})
	case g._config.AllowSSE:
		protocols = append(protocols, directoryResponseProtocol{
			Protocol: PROTOCOL_hsse01,
			Key:      q_key_http_sse,
		})
	case g._config.AllowWebsocket:
		protocols = append(protocols, directoryResponseProtocol{
			Protocol: PROTOCOL_webs01,
			Key:      q_key_websocket,
		})
	}
	dr.Protocols = protocols

	directory_response, err := json.Marshal(dr)
	if err != nil {
		panic(err)
	}

	g._directory_response = directory_response
	g._directory_response_len = strconv.Itoa(len(directory_response))
}

func (g *Starlight) directoryHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Server", "nginx")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Length", g._directory_response_len)
	w.WriteHeader(http.StatusOK)

	w.Write(g._directory_response)
}
