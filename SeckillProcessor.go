package main

import (
	"fmt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"jdseckill/utils"
	"time"
)

func CmJdMaotaiProcessor(cookiesId string) error {
	//TODO 初始化JdUtils
	jd := utils.NewJdUtils(cookiesId)

	//TODO 验证是否登录，未登录扫码登录
	if err := jd.LoginByQCode(); err != nil {
		logs.Error(err.Error())
		return err
	}

	//TODO 获取用户名称
	if err := jd.GetUserName(); err != nil {
		return nil
	}

	//TODO 获取商品名称
	if err := jd.GetSkuTitle(); err != nil {
		logs.Error(err.Error())
		return err
	}

	//TODO 获取商品价格
	if err := jd.GetPrice(); err != nil {
		logs.Error(err.Error())
		return err
	}
	weChatMessage := fmt.Sprintf(utils.MessageFormat, jd.UserName, jd.BuyTime, jd.SkuName, jd.SkuPrice, "成功", "未开始", "")
	if utils.AppConfig.MessageEnable {
		go jd.WeChatSendMessage(weChatMessage)
	}
	//TODO 预约商品
	if err := jd.CommodityAppointment(); err != nil {
		logs.Error(err.Error())
		return err
	}

	//TODO 定时任务，到达指定时间返回
	if err := jd.TaskCorn(); err != nil {
		return err
	}

	weChatMessage = fmt.Sprintf(utils.MessageFormat, jd.UserName, jd.BuyTime, jd.SkuName, jd.SkuPrice, "成功", "开始抢购", "")
	if utils.AppConfig.MessageEnable {
		go jd.WeChatSendMessage(weChatMessage)
	}

	//TODO 访问商品的抢购链接
	if err := jd.RequestSeckill(); err != nil {
		logs.Error(err.Error())
		return err
	}

	fastmodel := beego.AppConfig.DefaultBool("fastmodel", false)
	if !fastmodel {
		//TODO 访问抢购订单结算页面
		if err := jd.RequestCheckOut(); err != nil {
			logs.Error(err.Error())
			return err
		}
	}

	for {
		//TODO 开始提交订单
		if err := jd.SubmitOrder(); err == nil {
			return err
		}
		nowTime := time.Now()
		if nowTime.Sub(jd.BuyTime).Minutes() > utils.AppConfig.StopMinutes {
			logs.Info("抢购时间以过【%f】分钟，自动停止...", utils.AppConfig.StopMinutes)
			weChatMessage = fmt.Sprintf(utils.MessageFormat, jd.UserName, jd.BuyTime, jd.SkuName, jd.SkuPrice, "成功", "抢购失败", "")
			if utils.AppConfig.MessageEnable {
				go jd.WeChatSendMessage(weChatMessage)
			}
			return nil
		}
	}
}
