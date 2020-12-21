package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"jdseckill/utils/httplib"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/astaxie/beego/logs"
	"github.com/bitly/go-simplejson"
)

var (
	DefaultHeaders = map[string]string{
		"User-Agent":      "",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3",
		"ContentType":     "application/json",
		"Connection":      "keep-alive",
		"Accept-Encoding": "gzip, deflate",
		"Accept-Language": "zh-CN,zh;q=0.8",
	}

	TimeFormat = "2006-01-02 15:04:05.000000000"
	DayFormat  = "2006-01-02"
)

var (
	MessageFormat = "# 京东抢购商品详情\n- 抢购用户：%s\n- 抢购时间：%s\n- 商品名称：%s\n- 商品价格：%s\n- 登录状态：%s\n- 抢购状态：%s\n- 订单编号：%s\n"
)

type JdUtils struct {
	QrFilePath  string
	CookiesFile string
	Token       string
	UserName    string
	SkuName     string
	SkuPrice    string
	IsSleep     bool
	BuyTime     time.Time
	Jar         *SimpleJar
	CookiesId   string
}

func NewJdUtils(cookiesId string) *JdUtils {
	// make the folder to contain the resulting archive
	// if it does not already exist
	dir, _ := os.Getwd()
	cookieFileName := path.Join(dir, "cookies", cookiesId+".cookies")
	qrCodeFile := path.Join(dir, "images", cookiesId+".qr")
	destDir := filepath.Dir(cookieFileName)
	if !fileExists(destDir) {
		err := mkdir(destDir, 0755)
		if err != nil {
			logs.Error("making folder for cookies: %v", err)
		}
	}

	destDir = filepath.Dir(qrCodeFile)
	if !fileExists(destDir) {
		err := mkdir(destDir, 0755)
		if err != nil {
			logs.Error("making folder for images: %v", err)
		}
	}

	jd := &JdUtils{QrFilePath: qrCodeFile, CookiesFile: cookieFileName, CookiesId: cookiesId}
	jd.Jar = NewSimpleJar(JarOption{
		JarType:  JarJson,
		Filename: cookieFileName,
	})

	buyTime, err := time.ParseInLocation(TimeFormat, AppConfig.BuyTime, time.Local)
	if err != nil {
		logs.Error("时间转换异常：", err)
		os.Exit(0)
	}
	jd.BuyTime = buyTime
	if AppConfig.ValidateCookies {
		if err := jd.Jar.Load(); err != nil {
			logs.Info("加载Cookies失败:", err)
			jd.Jar.Clean()
		}
	} else {
		jd.Jar.Clean()
	}
	httplib.SetCookieJar(jd.Jar)
	return jd
}

/*==================================================Header=============================================================*/

func (jd *JdUtils) CustomHeader(req *http.Request, header map[string]string) {
	if req == nil || len(header) == 0 {
		return
	}
	for key, val := range header {
		req.Header.Set(key, val)
	}
}

/*==================================================Common=============================================================*/
func (jd *JdUtils) RunCommand(command string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", command)
	case "linux":
		cmd = exec.Command("eog", command)
	default:
		cmd = exec.Command("open", command)
	}
	if err := cmd.Start(); err != nil {
		if runtime.GOOS == "linux" {
			cmd = exec.Command("gnome-open", command)
			return cmd.Start()
		}
		return err
	}
	return nil
}

func ToJSON(respMsg string) (*simplejson.Json, error) {
	jsonStr := strings.ReplaceAll(respMsg, "\n", "")
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" || jsonStr == "null" {
		return nil, fmt.Errorf("原始字符串%s不是Json格式，处理后字符串%s ", respMsg, jsonStr)
	}

	buffer := []byte(jsonStr)
	validJson := json.Valid(buffer)
	if validJson {
		js, err := simplejson.NewJson(buffer)
		if err != nil {
			return nil, err
		}
		return js, nil
	} else {
		n1 := strings.Index(jsonStr, "(")
		n2 := strings.Index(jsonStr, ")")
		if n1 > -1 && n2 > -1 {
			jsonStr = jsonStr[n1+1 : n2]
		}
		if jsonStr == "" || jsonStr == "null" {
			return nil, fmt.Errorf("原始字符串%s不是Json格式，处理后字符串%s ", respMsg, jsonStr)
		}
		buffer = []byte(jsonStr)
		validJson = json.Valid(buffer)
		if validJson {
			js, err := simplejson.NewJson(buffer)
			if err != nil {
				return nil, err
			}
			return js, nil
		} else {
			return nil, fmt.Errorf("原始字符串%s不是Json格式，处理后字符串%s ", respMsg, jsonStr)
		}
	}
}

func (jd *JdUtils) Release() {
	//TODO 是否保存Cookies
	jd.SaveCookies()
	//TODO 删除图片
	DeleteFile(jd.QrFilePath)
}

// Release the resource opened
func (jd *JdUtils) SaveCookies() error {
	if !AppConfig.ValidateCookies {
		return nil
	}
	if jd.Jar != nil {
		err := jd.Jar.Persist()
		if err != nil {
			logs.Error("保存Cookies %s 失败: ", jd.Jar.filename, err)
			return err
		} else {
			logs.Info("保存Cookies %s 成功", jd.Jar.filename)
		}
	} else {
		logs.Info("Cookies 为空")
	}
	return nil
}

func (jd *JdUtils) AllowRedirects(req *httplib.BeegoHTTPRequest) {
	req.SetCheckRedirect(func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	})
}

/*==================================================Login==============================================================*/
func (jd *JdUtils) LoginByQCode() error {
	defer jd.Release()
	if AppConfig.ValidateCookies {
		logs.Info("开始验证Cookies 登录...")
		if jd.ValidateLogin() {
			return nil
		} else {
			jd.Jar.Clean()
		}
	} else {
		jd.Jar.Clean()
	}

	logs.Info("开始扫码登录...")
	if err := jd.LoginPage(); err != nil {
		return err
	}

	if err := jd.LoadQRCode(); err != nil {
		return err
	}

	logs.Info("QR Image:", jd.QrFilePath)
	// just start, do not wait it complete
	if err := jd.RunCommand(jd.QrFilePath); err != nil {
		logs.Info("打开二维码图片失败: %+v.", err)
		return err
	}

	for retry := 85; retry != 0; retry-- {
		err := jd.WaitForScan()
		if err == nil {
			break
		}
	}

	if err := jd.ValidateQRToken(); err != nil {
		return err
	}
	return nil
}

//https://order.jd.com/lazy/isPlusMember.action
func (jd *JdUtils) ValidateLogin() bool {
	url := "https://order.jd.com/center/list.action"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	req.Param("rid", strconv.FormatInt(time.Now().Unix()*1000, 10))
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	jd.AllowRedirects(req)
	resp, err := req.Response()
	if err != nil {
		logs.Info("需要重新登录: %+v", err)
		return false
	}

	if resp.StatusCode != http.StatusOK {
		logs.Info("需要重新登录")
		return false
	}
	logs.Info("Coolies登录成功，无需重新登录")
	return true
}

func (jd *JdUtils) LoginPage() error {
	url := "https://passport.jd.com/new/login.aspx"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	_, err := req.Response()
	if err != nil {
		logs.Error("登录页面请求异常:", err)
		return err
	}
	return nil
}

func (jd *JdUtils) LoadQRCode() error {
	url := "https://qr.m.jd.com/show"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	req.Param("appid", strconv.Itoa(133))
	req.Param("size", strconv.Itoa(147))
	req.Param("t", strconv.FormatInt(time.Now().Unix()*1000, 10))
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.Header("Referer", "https://passport.jd.com/new/login.aspx")
	resp, err := req.Response()
	if err != nil {
		logs.Error("加载登录二维码请求异常:", err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		logs.Error("加载登录二维码异常 http status : %d/%s", resp.StatusCode, resp.Status)
		err = fmt.Errorf("加载登录二维码异常 http status : %d/%s", resp.StatusCode, resp.Status)
		return err
	}
	filename := jd.QrFilePath + ".png"
	err = req.ToFile(filename)
	if err != nil {
		logs.Error("下载二维码失败: %+v", err)
		return err
	}
	jd.QrFilePath = filename
	return nil
}

func (jd *JdUtils) WaitForScan() error {
	url := "https://qr.m.jd.com/check"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	rand.Seed(time.Now().UnixNano())
	randomNumber := rand.Intn(9999999) + 1000000
	req.Param("callback", fmt.Sprintf("jQuery%d", randomNumber))
	req.Param("appid", strconv.Itoa(133))
	req.Param("token", jd.Jar.Get("wlfstk_smdl"))
	req.Param("_", strconv.FormatInt(time.Now().Unix()*1000, 10))
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.Header("Referer", "https://passport.jd.com/new/login.aspx")
	req.SetHost("qr.m.jd.com")

	resp, err := req.Response()
	if err != nil {
		logs.Error("验证二维码是否扫描请求异常：", err)
		return err
	}
	if resp.StatusCode == http.StatusOK {
		respMsg, err := req.String()
		if err != nil {
			logs.Error("获取扫码验证,请求数据异常：", err)
			return err
		}

		Json, err := ToJSON(respMsg)
		if err != nil {
			logs.Error("解析Json响应数据失败: %s ", err)
			return err
		}
		code := Json.Get("code").MustInt()
		if code == 200 {
			jd.Token = Json.Get("ticket").MustString()
			logs.Info("已完成手机客户端确认")
			logs.Info("Token : %+v", jd.Token)
		} else {
			logs.Info("%+v : %s", code, Json.Get("msg").MustString())
			time.Sleep(time.Second * 1)
		}
	}

	if jd.Token == "" {
		err := fmt.Errorf("未检测到QR扫码结果")
		return err
	}
	return nil
}

func (jd *JdUtils) ValidateQRToken() error {
	url := "https://passport.jd.com/uc/qrCodeTicketValidation"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	req.Header("Referer", "https://passport.jd.com/uc/login?ltype=logout")
	req.Param("t", jd.Token)
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	resp, err := req.Response()
	if err != nil {
		logs.Info("验证扫码Token请求异常：", err)
		return err
	}
	//
	// 京东有时候会认为当前登录有危险，需要手动验证
	// url: https://safe.jd.com/dangerousVerify/index.action?username=...
	//
	if resp.Header.Get("P3P") == "" {
		var res struct {
			ReturnCode int    `json:"returnCode"`
			Token      string `json:"token"`
			URL        string `json:"url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
			if res.URL != "" {
				verifyURL := res.URL
				if !strings.HasPrefix(verifyURL, "https:") {
					verifyURL = "https:" + verifyURL
				}
				logs.Error(2, "安全验证: %s", verifyURL)
				jd.RunCommand(verifyURL)
			}
		}
		return fmt.Errorf("登录失败...")
	}

	if resp.StatusCode == http.StatusOK {
		respMsg, err := req.String()
		if err != nil {
			logs.Error("获取扫码验证Token,请求数据异常: ", err)
			return err
		}
		Json, err := ToJSON(respMsg)
		if err != nil {
			logs.Error("解析Json响应数据失败: %s ", err)
			return err
		}
		code := Json.Get("returnCode").MustInt()
		if code == 0 {
			logs.Info("扫码登陆成功, P3P: %s", resp.Header.Get("P3P"))
			return nil
		} else {
			err = fmt.Errorf("%+v : %s", code, Json.Get("msg").MustString())
			logs.Error(err.Error())
			return err
		}
	} else {
		logs.Info("登陆失败")
		err = fmt.Errorf("%+v", resp.Status)
		return err
	}
}

func (jd *JdUtils) GetUserName() error {
	url := "https://passport.jd.com/user/petName/getUserInfoForMiniJd.action"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	rand.Seed(time.Now().UnixNano())
	randomNumber := rand.Intn(9999999) + 1000000
	req.Param("callback", fmt.Sprintf("jQuery%d", randomNumber))
	req.Param("_", strconv.FormatInt(time.Now().Unix()*1000, 10))
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.Header("Referer", "https://order.jd.com/center/list.action")
	resp, err := req.Response()
	if err != nil {
		logs.Error("获取用户名请求异常: ", err)
		return err
	}
	if resp.StatusCode == http.StatusOK {
		respMsg, err := req.String()
		if err != nil {
			logs.Error("获取用户名,请求数据异常: ", err)
			return err
		}
		Json, err := ToJSON(respMsg)
		if err != nil {
			return err
		}
		jd.UserName = Json.Get("nickName").MustString("jd")
		if jd.UserName == "" {
			return fmt.Errorf("Cookies 验证失败，获取用户名为空")
		}
		logs.Info("用户名：", jd.UserName)
		return nil
	} else {
		logs.Info("登陆失败")
		err = fmt.Errorf("%+v", resp.Status)
		return err
	}
}

func (jd *JdUtils) GetSkuTitle() error {
	url := fmt.Sprintf("https://item.jd.com/%s.html", AppConfig.SkuId)
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.Header("Referer", "https://mall.jd.com")
	resp, err := req.Response()
	if err != nil {
		logs.Error("获取商品名称请求异常: ", err)
		return err
	}
	if resp.StatusCode == http.StatusOK {
		respMsg, err := req.Bytes()
		if err != nil {
			logs.Error("获取商品名称数据异常: ", err)
			return err
		}
		doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(respMsg))
		if err != nil {
			logs.Error(0, "解析商品页面失败: %+v", err)
			return err
		}
		jd.SkuName = strings.Trim(doc.Find("div.sku-name").Text(), " \t\n")
		logs.Info("商品名称：", jd.SkuName)
		return nil
	} else {
		logs.Error("获取商品名称异常: ", err)
		err = fmt.Errorf("%+v", resp.Status)
		return err
	}
}

func (jd *JdUtils) GetPrice() error {
	url := "http://p.3.cn/prices/mgets"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	req.Param("type", "1")
	req.Param("skuIds", "J_"+AppConfig.SkuId)
	req.Param("pduid", strconv.FormatInt(time.Now().Unix()*1000, 10))
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	resp, err := req.Response()
	if err != nil {
		logs.Error("获取商品（%s）价格请求异常: %", AppConfig.SkuId, err)
		return err
	}

	if resp.StatusCode == http.StatusOK {
		respMsg, err := req.String()
		if err != nil {
			logs.Error("获取商品价格,请求数据异常: ", err)
			return err
		}
		Json, err := ToJSON(respMsg)
		if err != nil {
			logs.Error("解析Json响应数据失败: %s ", err)
			return err
		}
		price, err := Json.GetIndex(0).Get("p").String()
		if err != nil {
			logs.Error("获取商品（%s）价格失败: %", AppConfig.SkuId, err)
			return err
		}
		jd.SkuPrice = price
		logs.Info("商品价格：", jd.SkuPrice)
		return nil
	} else {
		logs.Error("获取商品（%s）价格失败: %", AppConfig.SkuId, err)
		err = fmt.Errorf("%+v", resp.Status)
		return err
	}
}

func (jd *JdUtils) ResponseJdHome() error {
	logs.Info("访问Jd主页...")
	url := "https://www.jd.com"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.Header("Referer", "https://passport.jd.com")
	req.SetHost("www.jd.com")
	resp, err := req.Response()
	if err != nil {
		logs.Error("访问Jd主页请求异常: ", err)
		return err
	}
	if resp.StatusCode == http.StatusOK {
		logs.Info("访问Jd主页链接OK")
		return nil
	} else {
		err := fmt.Errorf("访问Jd主页链接失败StatusCode: %d", resp.StatusCode)
		logs.Error(err.Error())
		return err
	}
}

/*==================================================Seckill============================================================*/
func (jd *JdUtils) RequestSeckill() error {
	logs.Info("访问商品的抢购连接...")
	skuUrl := fmt.Sprintf("https://item.jd.com/%s.html", AppConfig.SkuId)
	url := jd.GetSeckillUrl()
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.Header("Referer", skuUrl)
	req.SetHost("marathon.jd.com")
	jd.AllowRedirects(req)
	resp, err := req.Response()
	if err != nil {
		logs.Error("访问商品的抢购链接请求异常: ", err)
		return err
	}

	if resp.StatusCode == http.StatusFound {
		logs.Info("访问商品的抢购链接OK")
		return nil
	} else {
		err := fmt.Errorf("访问商品的抢购链接失败StatusCode: %d", resp.StatusCode)
		logs.Error(err.Error())
		return err
	}
}

func (jd *JdUtils) GetSeckillUrl() string {
	//TODO 获取商品的抢购链接
	//	点击"抢购"按钮后，会有两次302跳转，最后到达订单结算页面
	//	这里返回第一次跳转后的页面url，作为商品的抢购链接 :return: 商品的抢购链接
	for {
		skuUrl := fmt.Sprintf("https://item.jd.com/%s.html", AppConfig.SkuId)
		url := "https://itemko.jd.com/itemShowBtn"
		req := httplib.Get(url)
		req.SetEnableCookie(true)
		rand.Seed(time.Now().UnixNano())
		randomNumber := rand.Intn(9999999) + 1000000
		req.Param("callback", fmt.Sprintf("jQuery%d", randomNumber))
		req.Param("skuId", AppConfig.SkuId)
		req.Param("from", "pc")
		req.Param("_", strconv.FormatInt(time.Now().Unix()*1000, 10))
		DefaultHeaders["User-Agent"] = AppConfig.UserAgent
		jd.CustomHeader(req.GetRequest(), DefaultHeaders)
		req.Header("Referer", skuUrl)
		req.SetHost("itemko.jd.com")
		resp, err := req.Response()
		if err != nil {
			logs.Error("获取商品的抢购链接请求异常: ", err)
			if jd.IsSleep {
				time.Sleep(time.Duration(10) * time.Microsecond)
			}
			continue
		}
		if resp.StatusCode == http.StatusOK {
			respMsg, err := req.String()
			if err != nil {
				logs.Error("获取商品的抢购链接,请求数据异常: ", err)
				if jd.IsSleep {
					time.Sleep(time.Duration(10) * time.Microsecond)
				}
				continue
			}
			Json, err := ToJSON(respMsg)
			if err != nil {
				logs.Error("解析Json响应数据失败: %s ", err)
				if jd.IsSleep {
					time.Sleep(time.Duration(10) * time.Microsecond)
				}
				continue
			}
			routeUrl := Json.Get("url").MustString("")
			if routeUrl == "" {
				logs.Info("抢购链接获取失败，%s 不是抢购商品或抢购页面暂未刷新，稍后重试...", AppConfig.SkuId)
				if jd.IsSleep {
					time.Sleep(time.Duration(10) * time.Microsecond)
				}
				continue
			} else {
				//TODO https://divide.jd.com/user_routing?skuId=8654289&sn=c3f4ececd8461f0e4d7267e96a91e0e0&from=pc
				routeUrl = fmt.Sprintf("https:%s", routeUrl)
				//TODO https://marathon.jd.com/captcha.html?skuId=8654289&sn=c3f4ececd8461f0e4d7267e96a91e0e0&from=pc
				seckillUrl := strings.ReplaceAll(routeUrl, "divide", "marathon")
				seckillUrl = strings.ReplaceAll(seckillUrl, "user_routing", "captcha.html")
				logs.Info("抢购链接获取成功:", seckillUrl)
				return seckillUrl
			}
		}
		if jd.IsSleep {
			time.Sleep(time.Duration(10) * time.Microsecond)
		}
	}
}

func (jd *JdUtils) RequestCheckOut() error {
	logs.Info("访问抢购订单结算页面...")
	skuUrl := fmt.Sprintf("https://item.jd.com/%s.html", AppConfig.SkuId)
	url := "https://marathon.jd.com/seckill/seckill.action"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	req.Param("skuId", AppConfig.SkuId)
	req.Param("num", strconv.FormatInt(AppConfig.CheckOutNumber, 10))
	req.Param("rid", strconv.FormatInt(time.Now().Unix()*1000, 10))
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.Header("Referer", skuUrl)
	req.SetHost("marathon.jd.com")
	jd.AllowRedirects(req)
	resp, err := req.Response()
	if err != nil {
		logs.Error("访问抢购订单结算页面请求异常: ", err)
		return err
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusFound {
		logs.Info("访问抢购订单结算页链接OK")
		return nil
	} else {
		err := fmt.Errorf("访问抢购订单结算失败StatusCode: %d", resp.StatusCode)
		logs.Error(err.Error())
		return err
	}
}

func (jd *JdUtils) SubmitOrder() error {
	orderData, err := jd.GetOrderData()
	if err != nil {
		logs.Error("生成提交抢购订单所需参数异常：", err)
		return err
	}
	for i := 0; i < 100; i++ {
		go logs.Info("开始提交抢购订单【%d】次...", i)
		url := "https://marathon.jd.com/seckillnew/orderService/pc/submitOrder.action"
		skillUrl := fmt.Sprintf("https://marathon.jd.com/seckill/seckill.action?skuId=%s&num=%s&rid=%s", AppConfig.SkuId, strconv.FormatInt(AppConfig.CheckOutNumber, 10), strconv.FormatInt(time.Now().Unix()*1000, 10))
		req := httplib.Post(url)
		req.SetEnableCookie(true)
		req.Param("skuId", AppConfig.SkuId)
		req.JSONBody(orderData)
		DefaultHeaders["User-Agent"] = AppConfig.UserAgent
		jd.CustomHeader(req.GetRequest(), DefaultHeaders)
		req.Header("Referer", skillUrl)
		req.SetHost("marathon.jd.com")
		resp, err := req.Response()
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			respMsg, err := req.String()
			if err != nil {
				logs.Error("获取抢购订单,请求数据异常: ", err)
				if jd.IsSleep {
					time.Sleep(time.Duration(10) * time.Microsecond)
				}
				continue
			}
			Json, err := ToJSON(respMsg)
			if err != nil {
				logs.Error("解析Json响应数据失败: %s ", err)
				continue
			}
			orderStatus := Json.Get("success").MustBool(false)
			if orderStatus {
				orderId := Json.Get("orderId")
				totalMoney := Json.Get("totalMoney")
				payUrl := Json.Get("pcUrl")
				logs.Info("抢购成功，订单号:%s, 总价:%s, 电脑端付款链接:%s", orderId, totalMoney, payUrl)
				weChatMessage := fmt.Sprintf(MessageFormat, jd.UserName, jd.BuyTime, jd.SkuName, jd.SkuPrice, "成功", "抢购成功", orderId)
				if AppConfig.MessageEnable {
					go jd.WeChatSendMessage(weChatMessage)
				}
				return nil
			} else {
				logs.Error("抢购失败，返回信息:%s", respMsg)
				continue
			}
		} else {
			err := fmt.Errorf("提交抢购订单失败,StatusCode: %d", resp.StatusCode)
			logs.Error(err.Error())
			continue
		}
	}
	return fmt.Errorf("抢购失败，即将重试...")
}

func (jd *JdUtils) GetOrderData() (map[string]interface{}, error) {
	logs.Info("生成提交抢购订单所需参数...")
	jsonOrder, err := jd.GetOrderInitData()
	if err != nil {
		logs.Error(err.Error())
		return nil, err
	}
	//TODO 默认地址dict
	defaultAddress := jsonOrder.Get("addressList").GetIndex(0)
	//TODO 默认发票信息dict, 有可能不返回
	invoiceInfo := jsonOrder.Get("invoiceInfo")
	//TODO ToKen
	token := jsonOrder.Get("token")
	orderdata := make(map[string]interface{})
	orderdata["skuId"] = AppConfig.SkuId
	orderdata["num"] = AppConfig.SubmitOrderNumber
	orderdata["addressId"] = defaultAddress.Get("id")
	orderdata["yuShou"] = "true"
	orderdata["isModifyAddress"] = "false"
	orderdata["name"] = defaultAddress.Get("name")
	orderdata["provinceId"] = defaultAddress.Get("provinceId")
	orderdata["cityId"] = defaultAddress.Get("cityId")
	orderdata["countyId"] = defaultAddress.Get("countyId")
	orderdata["townId"] = defaultAddress.Get("townId")
	orderdata["addressDetail"] = defaultAddress.Get("addressDetail")
	orderdata["mobile"] = defaultAddress.Get("mobile")
	orderdata["mobileKey"] = defaultAddress.Get("mobileKey")
	orderdata["email"] = defaultAddress.Get("email").MustString("")
	orderdata["postCode"] = ""
	orderdata["invoiceTitle"] = invoiceInfo.Get("invoiceTitle").MustInt(-1)
	orderdata["invoiceCompanyName"] = ""
	orderdata["invoiceContent"] = invoiceInfo.Get("invoiceContentType").MustInt(1)
	orderdata["invoiceTaxpayerNO"] = ""
	orderdata["invoiceEmail"] = ""
	orderdata["invoicePhone"] = invoiceInfo.Get("invoicePhone").MustString("")
	orderdata["invoicePhoneKey"] = invoiceInfo.Get("invoicePhoneKey").MustString("")
	orderdata["invoice"] = "true"
	orderdata["password"] = ""
	orderdata["codTimeType"] = 3
	orderdata["paymentType"] = 4
	orderdata["areaCode"] = ""
	orderdata["overseas"] = 0
	orderdata["phone"] = ""
	orderdata["eid"] = AppConfig.Eid
	orderdata["fp"] = AppConfig.Fp
	orderdata["token"] = token
	orderdata["pru"] = ""
	return orderdata, nil
}

func (jd *JdUtils) GetOrderInitData() (*simplejson.Json, error) {
	logs.Info("获取秒杀初始化信息（包括：地址，发票，token）")
	url := "https://marathon.jd.com/seckillnew/orderService/pc/init.action"
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	req.Param("sku", AppConfig.SkuId)
	req.Param("num", strconv.FormatInt(AppConfig.OrderInfoNumber, 10))
	req.Param("isModifyAddress", "false")
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.SetHost("marathon.jd.com")
	for i := 0; i < 100; i++ {
		resp, err := req.Response()
		if err != nil {
			logs.Error("获取秒杀初始化信息请求异常: ", err)
			if jd.IsSleep {
				time.Sleep(time.Duration(10) * time.Microsecond)
			}
			continue
		}
		if resp.StatusCode == http.StatusOK {
			respMsg, err := req.String()
			if err != nil {
				logs.Error("获取秒杀初始化信息,请求数据异常: ", err)
				if jd.IsSleep {
					time.Sleep(time.Duration(10) * time.Microsecond)
				}
				continue
			}

			Json, err := ToJSON(respMsg)
			if err != nil {
				logs.Error("解析Json响应数据失败: %s ", err)
				if jd.IsSleep {
					time.Sleep(time.Duration(10) * time.Microsecond)
				}
				continue
			}
			return Json, nil
		} else {
			err := fmt.Errorf("获取秒杀初始化信息失败,StatusCode: %d", resp.StatusCode)
			logs.Error(err.Error())
			if jd.IsSleep {
				time.Sleep(time.Duration(10) * time.Microsecond)
			}
			continue
		}
	}
	return nil, fmt.Errorf("获取秒杀初始化信息失败")
}

/*================================================CommodityAppointment=================================================*/

func (jd *JdUtils) CommodityAppointment() error {
	reservationUrl, err := jd.GetReservationUrlUrl()
	if err != nil {
		return err
	}
	req := httplib.Get(reservationUrl)
	req.SetEnableCookie(true)
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	resp, err := req.Response()
	if err != nil {
		logs.Error("预约商品请求异常: ", err)
		return err
	}
	if resp.StatusCode == http.StatusOK {
		logs.Info("预约成功，已获得抢购资格 / 您已成功预约过了，无需重复预约")
		return nil
	} else {
		err := fmt.Errorf("预约商品失败,StatusCode: %d", resp.StatusCode)
		logs.Error(err.Error())
		return err
	}
}

func (jd *JdUtils) GetReservationUrlUrl() (string, error) {
	url := "https://yushou.jd.com/youshouinfo.action?"
	skuUrl := fmt.Sprintf("https://item.jd.com/%s.html", AppConfig.SkuId)
	req := httplib.Get(url)
	req.SetEnableCookie(true)
	req.Param("callback", "fetchJSON")
	req.Param("sku", AppConfig.SkuId)
	req.Param("_", strconv.FormatInt(time.Now().Unix()*1000, 10))
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	req.Header("Referer", skuUrl)
	resp, err := req.Response()
	if err != nil {
		logs.Error("获取预约商品链接请求异常: ", err)
		return "", err
	}
	if resp.StatusCode == http.StatusOK {
		respMsg, err := req.String()
		if err != nil {
			logs.Error("获取预约商品链接,请求数据异常: ", err)
			return "", err
		}
		Json, err := ToJSON(respMsg)
		if err != nil {
			logs.Error("解析Json响应数据失败: %s ", err)
			return "", err
		}
		reserveUrl := Json.Get("url").MustString()
		reserveUrl = fmt.Sprintf("https:%s", reserveUrl)
		return reserveUrl, nil
	} else {
		err := fmt.Errorf("获取预约商品链接失败,StatusCode: %d", resp.StatusCode)
		logs.Error(err.Error())
		return "", err
	}
}

/*================================================TaskCorn=============================================================*/
func (jd *JdUtils) TaskCorn() error {
	logs.Info("正在等待到达设定时间:", jd.BuyTime.String())
	for {
		nowTime := time.Now()
		if nowTime.Sub(jd.BuyTime).Minutes() > AppConfig.StopMinutes {
			message := fmt.Sprintf("抢购时间以过【%f】分钟，自动停止...", AppConfig.StopMinutes)
			logs.Info(message)
			return fmt.Errorf(message)
		}
		if nowTime.Before(jd.BuyTime) {
			time.Sleep(time.Duration(10) * time.Microsecond)
		} else {
			logs.Info("时间到达，开始执行……")
			return nil
		}
	}
}

/*================================================Wechat=============================================================*/
func (jd *JdUtils) WeChatSendMessage(message string) error {
	url := fmt.Sprintf("http://sc.ftqq.com/%s.send", AppConfig.MessageKey)
	req := httplib.Get(url)
	req.Param("text", "京东抢购通知")
	req.Param("desp", message)
	DefaultHeaders["User-Agent"] = AppConfig.UserAgent
	jd.CustomHeader(req.GetRequest(), DefaultHeaders)
	resp, err := req.Response()
	if err != nil {
		logs.Error("推送微信消息请求异常: ", err)
		return err
	}
	if resp.StatusCode == http.StatusOK {
		return nil
	} else {
		err := fmt.Errorf("推送微信消失败,StatusCode: %d", resp.StatusCode)
		logs.Error(err.Error())
		return err
	}
}
