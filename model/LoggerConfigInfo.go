package model

type LoggerConfigInfo struct {
	LogFileName string `json:"filename"`
	LogSeparate string `json:"separate"`
	LogLevel    int    `json:"level"`
	LogMaxlines int64  `json:"maxlines"`
	LogMaxsize  int64  `json:"maxsize"`
	LogMaxdays  int    `json:"maxdays"`
	LogDaily    bool   `json:"daily"`
	LogColor    bool   `json:"color"`
}

type AppConfig struct {
	LoggerConfigInfo  LoggerConfigInfo `json:"log"`
	Eid               string
	Fp                string
	SkuId             string
	BuyTime           string
	UserAgent         string
	RandomUserAgent   bool
	MessageEnable     bool
	MessageKey        string
	ValidateCookies   bool
	CheckOutNumber    int64
	SubmitOrderNumber int64
	OrderInfoNumber   int64
	StopSeconds       float64
	IsSleep           bool
	IsFast            bool
	SleepMillisecond  int64
}
