package model

type Server struct {
	Admin    string `json:"admin"`
	Email    string `json:"email"`
	OS       string `json:"os"`
	ID       string `json:"id"`
	Software string `json:"software"`
	Version  string `json:"version"`
	TimeNow  int64  `json:"timeNow"`
	Uptime   int64  `json:"uptime"`
}

type Return struct {
	Msg    string `json:"msg"`
	Server Server `json:"server"`
}
