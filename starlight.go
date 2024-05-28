package starlight

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
)

const PROTOCOL_VERSION = 1

type StarlightConfig struct {
	AllowPoll          bool // poll01
	AllowLongPoll      bool // lpll01
	AllowHTTPStreaming bool // ress01
	AllowWebsocket     bool // webs01

	BasePath        string
	DefaultProtocol string

	NotFoundHandler func(w http.ResponseWriter, r *http.Request)
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

func WithAllowHTTPStreaming(allow bool) StarlightOption {
	return func(c *StarlightConfig) {
		c.AllowHTTPStreaming = allow
	}
}

func WithAllowWebsocket(allow bool) StarlightOption {
	return func(c *StarlightConfig) {
		c.AllowWebsocket = allow
	}
}

func WithDefaultProtocol(protocol string) StarlightOption {
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
	rand.Read(s[:])

	var c = StarlightConfig{}
	for _, option := range options {
		option(&c)
	}

	config := &Starlight{
		_config:       c,
		_server_token: hex.EncodeToString(s[:]),
	}

	config.buildDirectoryResponse()

	return config
}

type Starlight struct {
	_config StarlightConfig

	_server_token           string
	_directory_response     []byte
	_directory_response_len string
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

const (
	q_key                         = "starlight"
	q_key_directory               = "directory"
	q_key_http_response_streaming = "f2ace8d571ac98ae" // ress01
	q_key_websocket               = "ee99a57a33ec9ca2" // webs01
	q_key_long_polling            = "e14abc88cb6c5dcb" // lpll01
	q_key_polling                 = "c29bb1b250b6d522" // poll01

	PROTOCOL_ress01 = "ress01"
	PROTOCOL_webs01 = "webs01"
	PROTOCOL_lpll01 = "lpll01"
	PROTOCOL_poll01 = "poll01"
)

func (s *Starlight) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get(q_key) {
	case q_key_directory:
		s.directoryHandler(w, r)
	case q_key_http_response_streaming:
		// s.httpResponseStreamingHandler(w, r)
	case q_key_websocket:
		// s.websocketHandler(w, r)
	case q_key_polling:
		// s.pollingHandler(w, r)
	default:
		if s._config.NotFoundHandler != nil {
			s._config.NotFoundHandler(w, r)
		}
	}
}

type directoryResponse struct {
	ProtocolVersion int                         `json:"version"`
	BasePath        string                      `json:"base_path"`
	ServerToken     string                      `json:"server_token"`
	Protocols       []directoryResponseProtocol `json:"protocols"`
	Preferred       string                      `json:"preferred"`
}

type directoryResponseProtocol struct {
	Protocol string `json:"protocol"`
	Key      string `json:"key"`
}

func (s *Starlight) buildDirectoryResponse() {
	dr := directoryResponse{
		ProtocolVersion: PROTOCOL_VERSION,
		ServerToken:     s._server_token,
		BasePath:        s._config.BasePath,
		Preferred:       s._config.DefaultProtocol,
	}

	var protocols []directoryResponseProtocol

	switch {
	case s._config.AllowPoll:
		protocols = append(protocols, directoryResponseProtocol{
			Protocol: PROTOCOL_poll01,
			Key:      q_key_polling,
		})
	case s._config.AllowLongPoll:
		protocols = append(protocols, directoryResponseProtocol{
			Protocol: PROTOCOL_lpll01,
			Key:      q_key_long_polling,
		})
	case s._config.AllowHTTPStreaming:
		protocols = append(protocols, directoryResponseProtocol{
			Protocol: PROTOCOL_ress01,
			Key:      q_key_http_response_streaming,
		})
	case s._config.AllowWebsocket:
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

	s._directory_response = directory_response
	s._directory_response_len = strconv.Itoa(len(directory_response))
}

func (s *Starlight) directoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", "nginx")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Length", s._directory_response_len)
	w.WriteHeader(http.StatusOK)

	w.Write(s._directory_response)
}
