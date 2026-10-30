package schemes

type Responses struct {
	StatusCode int         `json:"code"`
	Data       interface{} `json:"data"`
}

type EmptyResponses struct {
}
type MessageResponses struct {
	Message string `json:"message"`
}
