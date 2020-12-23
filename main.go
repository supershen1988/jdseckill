package main

import (
	"flag"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/nsf/termbox-go"
	"jdseckill/utils"
	"strconv"
	"time"
)

var (
	userId    = flag.String("i", "", "用户编号")
	fastModel = flag.Bool("f", false, "快速模式")
)

func init() {
	if err := termbox.Init(); err != nil {
		panic(err)
	}
	termbox.SetCursor(0, 0)
	termbox.HideCursor()
}

func pause() {
	logs.Info("请按任意键继续...")
Loop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			break Loop
		}
	}
}

func main() {
	flag.Parse()
	cookiesId := *userId
	isFast := *fastModel
	if cookiesId == "" {
		cookiesId = strconv.FormatInt(time.Now().Unix(), 10)
	}
	//TODO 初始化配置信息
	utils.InitAppConfigByJson(beego.AppConfig.String("logConf"), cookiesId)
	CmJdMaotaiProcessor(cookiesId, isFast || utils.AppConfig.IsFast)
	pause()
}
