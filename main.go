package main

import (
	"flag"
	"github.com/astaxie/beego"
	"jdseckill/utils"
	"strconv"
	"time"
)

var (
	userId = flag.String("i", "", "用户编号")
	fastModel = flag.Bool("f", false, "快速模式")
)

func main() {
	flag.Parse()
	cookiesId := *userId
	isFast:=*fastModel
	if cookiesId == "" {
		cookiesId = strconv.FormatInt(time.Now().Unix(), 10)
	}
	//TODO 初始化配置信息
	utils.InitAppConfigByJson(beego.AppConfig.String("logConf"), cookiesId)
	configFast := beego.AppConfig.DefaultBool("isFast", false)
	CmJdMaotaiProcessor(cookiesId, isFast||configFast)
}
