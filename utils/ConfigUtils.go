package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"jdseckill/model"
	"math/rand"
	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

var (
	AppConfig model.AppConfig
)

func InitAppConfigByJson(configPath, cookiesId string) {
	cdata, err := ioutil.ReadFile(configPath)
	if err != nil {
		logs.Error("read config file ["+configPath+"] err: ", err)
		return
	}
	err = json.Unmarshal(cdata, &AppConfig)
	if err != nil {
		logs.Error("parse config file ["+configPath+"] err: ", err)
		return
	}

	logConf := make(map[string]interface{})
	AppConfig.LoggerConfigInfo.LogFileName = fmt.Sprintf("logs/%s.log", cookiesId)
	mapJsonBytes, err := json.Marshal(AppConfig.LoggerConfigInfo)
	json.Unmarshal(mapJsonBytes, &logConf)
	logJsonBytes, err := json.Marshal(logConf)
	if err != nil {
		fmt.Println("MapToJsonDemo err: ", err)
	}
	logs.SetLogger(logs.AdapterMultiFile, string(logJsonBytes))
	logs.SetLogger("console")
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)

	eid := beego.AppConfig.String("eid")
	fp := beego.AppConfig.String("fp")
	skuId := beego.AppConfig.String("skuId")
	buyTime := beego.AppConfig.String("buyTime")
	userAgent := beego.AppConfig.String("userAgent")
	randomUseragent := beego.AppConfig.DefaultBool("randomUseragent", false)
	messengerEnable := beego.AppConfig.DefaultBool("messengerEnable", false)
	messengerSckey := beego.AppConfig.String("messengerSckey")

	validateCookies := beego.AppConfig.DefaultBool("SaveCookies", false)
	checkOutNumber := beego.AppConfig.DefaultInt64("CheckOutNumber", 2)
	submitOrderNumber := beego.AppConfig.DefaultInt64("SubmitOrderNumber", 1)
	orderInfoNumber := beego.AppConfig.DefaultInt64("OrderInfoNumber", 1)
	stopSeconds := beego.AppConfig.DefaultFloat("StopSeconds", 30)
	isFast := beego.AppConfig.DefaultBool("IsFast", false)
	isSleep := beego.AppConfig.DefaultBool("IsSleep", false)
	sleepMillisecond := beego.AppConfig.DefaultInt64("SleepMillisecond", 100)
	dayStr := time.Now().Format(DayFormat)
	if randomUseragent {
		rand.Seed(time.Now().Unix())
		userAgent = UserAgents[rand.Intn(len(UserAgents))]
	}
	AppConfig.Eid = eid
	AppConfig.Fp = fp
	AppConfig.SkuId = skuId
	AppConfig.BuyTime = fmt.Sprintf("%s %s", dayStr, buyTime)
	AppConfig.UserAgent = userAgent
	AppConfig.RandomUserAgent = randomUseragent
	AppConfig.MessageEnable = messengerEnable
	AppConfig.MessageKey = messengerSckey
	AppConfig.ValidateCookies = validateCookies
	AppConfig.CheckOutNumber = checkOutNumber
	AppConfig.SubmitOrderNumber = submitOrderNumber
	AppConfig.OrderInfoNumber = orderInfoNumber
	AppConfig.StopSeconds = stopSeconds
	AppConfig.IsSleep = isSleep
	AppConfig.IsFast = isFast
	AppConfig.SleepMillisecond = sleepMillisecond
}
