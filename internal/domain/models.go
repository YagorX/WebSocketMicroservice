package domain

type BatchItem struct {
	Request Request
	Future  chan *Response
}

type Request struct {
	UUID      string `json:"uuid"`
	ModelName string `json:"model_name"`
	Message   string `json:"message"`
}

type Response struct {
	UUID      string `json:"uuid"`
	Response  string `json:"response"`
	CreatedAt string `json:"created_at"`
}

type WSPing struct {
	Type string `json:"type"` // "ping"
}
type WSPong struct {
	Type string `json:"type"` // "pong"
}
