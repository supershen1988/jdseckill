package main

import (
	"flag"
	"github.com/astaxie/beego"
	"jdseckill/utils"
	"strconv"
	"time"
)

var(
	userId = flag.String("i", "", "用户编号")
)

func init() {
	//TODO 初始化配置信息
	utils.InitAppConfigByJson(beego.AppConfig.String("logConf"))
}

func main() {
	flag.Parse()
	cookiesId:=*userId
	if cookiesId==""{
		cookiesId=strconv.FormatInt(time.Now().Unix(), 10)
	}
	CmJdMaotaiProcessor(cookiesId)
}

