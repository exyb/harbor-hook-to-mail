package routes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/exyb/harbor-hook-to-mail/config"
	"github.com/exyb/harbor-hook-to-mail/handlers"
	"github.com/robfig/cron/v3"
	"golang.org/x/exp/rand"

	"github.com/gin-gonic/gin"
)

type Once struct {
	Sign       string
	CreateTime string
}

type HookStats struct {
	Name   string
	Calls  int32
	Errors int32
	Once   Once
}

var (
	hookStatsMap      sync.Map
	hookConfig        *HookConfig
	hookStatsJsonFile = "stats.json"
)

// var (
// 	hookStatsMap = make(map[string]*HookStats, 3)
// )

type WebhookRequest struct {
	Type      string `json:"type"`
	OccurAt   int64  `json:"occur_at"`
	Operator  string `json:"operator"`
	EventData struct {
		Resources []struct {
			Digest      string `json:"digest"`
			Tag         string `json:"tag"`
			ResourceURL string `json:"resource_url"`
		} `json:"resources"`
		Repository struct {
			DateCreated  int64  `json:"date_created"`
			Name         string `json:"name"`
			Namespace    string `json:"namespace"`
			RepoFullName string `json:"repo_full_name"`
			RepoType     string `json:"repo_type"`
		} `json:"repository"`
	} `json:"event_data"`
}

func SetupRouter(r *gin.Engine) {
	// /hook/{app,ui,...}
	// r.POST("/hook/*", wrappedHookHandler)
	// r.POST("/hook/{backend,front,core}", wrappedHookHandler)
	hookConfig := getHookConfig()
	for _, app := range hookConfig.Hook.Apps {
		getOrCreateHookStats(app)
	}

	if err := LoadMapFromFile(); err != nil {
		log.Fatalf("Failed to load map from file: %v", err)
	}
	log.Println("Print content of saved stats afer loaded from file")
	PrintHookStatsMap()

	r.POST(hookConfig.Hook.ContextPath, webHookHandler)
	// hookGroup := r.Group("/hook")
	// {
	// 	hookGroup.POST("/:app", wrappedHookHandler)
	// }

	go resetStatCounters()
	go informHookStatsByCronExpr()
	go informHookStatsByExactTime()
	go saveHookStatsToFile()

}

// PrintHookStatsMap 打印 sync.Map 的内容
func PrintHookStatsMap() {
	hookStatsMap.Range(func(key, value interface{}) bool {
		fmt.Printf("%s: %v\n", key, value)
		return true
	})
}

func PrintMap(mapObject map[string]interface{}) bool {
	for key, value := range mapObject {
		fmt.Printf("%s: %v\n", key, value)
		return true
	}
	return false
}

func saveHookStatsToFile() {
	for {
		if err := SaveMapToFile(); err != nil {
			fmt.Printf("Error saving map to file: %v\n", err)
		}
		time.Sleep(time.Minute * 1)
	}
}

func getAppName(path string) string {
	parts := strings.Split(strings.Split(path, ":")[0], "/")
	if len(parts) < 3 {
		return ""
	}
	return parts[len(parts)-1]
}

func getTimeFromTag(tagName string) string {
	parts := strings.Split(tagName, "_")
	if len(parts) < 2 {
		return time.Now().Format("20060102150405")
	}
	return parts[len(parts)-1]
}

func getHookConfigFromPath(configFilePath string) *HookConfig {
	if hookConfig != nil {
		return hookConfig
	}
	hookConfig, err := LoadHookConfig(configFilePath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return hookConfig
}

func getHookConfig() *HookConfig {
	if hookConfig != nil {
		return hookConfig
	}
	configFilePath := os.Getenv("config_file_path")
	return getHookConfigFromPath(configFilePath)
}

func getOrCreateHookStats(app string) *HookStats {
	var stats *HookStats
	value, ok := hookStatsMap.Load(app)
	// load previous status Result

	if !ok {
		stats = &HookStats{
			Name:   app,
			Calls:  0,
			Errors: 0,
			Once: Once{
				Sign:       "",
				CreateTime: "20060101000000",
			},
		}
		_, _ = hookStatsMap.LoadOrStore(app, stats)
		return stats
	}
	// 第一次是一个 结构体, 第二次就是具体对象了
	hookStats, ok := value.(*HookStats)
	if ok {
		return hookStats
	}
	// 类型断言, 从 any 转为 map
	data, ok := value.(map[string]interface{})
	if ok {
		// 类型转换
		hookStats, err := convertToHookStats(data)
		if err != nil {
			fmt.Println("Error converting to HookStats:", err)
			return nil
		}

		return &hookStats
	}
	return nil

}

func convertToHookStats(data map[string]interface{}) (HookStats, error) {
	onceMap, ok := data["Once"].(map[string]interface{})
	if !ok {
		return HookStats{}, fmt.Errorf("invalid type for Once field")
	}

	createTime, ok := onceMap["CreateTime"].(string)
	if !ok {
		return HookStats{}, fmt.Errorf("invalid type for CreateTime field")
	}

	sign, ok := onceMap["Sign"].(string)
	if !ok {
		return HookStats{}, fmt.Errorf("invalid type for Sign field")
	}

	once := Once{
		CreateTime: createTime,
		Sign:       sign,
	}

	return HookStats{
		Name:   data["Name"].(string),
		Calls:  int32(data["Calls"].(float64)),
		Errors: int32(data["Errors"].(float64)),
		Once:   once,
	}, nil
}

func SaveMapToFile() error {
	data := make(map[string]interface{})

	// 将 sync.Map 的内容复制到普通 map
	hookStatsMap.Range(func(key, value interface{}) bool {
		data[key.(string)] = value
		return true
	})

	// 序列化为 JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// 写入文件
	return ioutil.WriteFile(hookStatsJsonFile, jsonData, 0644)
}

// LoadMapFromFile 从文件中读取并恢复到 sync.Map
func LoadMapFromFile() error {
	file, err := os.Open(hookStatsJsonFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在不是错误
		}
		return err
	}
	defer file.Close()

	jsonData, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	data := make(map[string]interface{})
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	// 将数据恢复到 sync.Map
	for key, value := range data {
		data, ok := value.(map[string]interface{})
		if ok {
			// 类型转换
			hookStats, err := convertToHookStats(data)
			if err != nil {
				fmt.Println("Error converting to HookStats:", err)
				return nil
			}

			hookStatsMap.Store(key, &hookStats)
		}
	}

	return nil
}
func getHookSign(app string) Once {
	stats := getOrCreateHookStats(app)
	return stats.Once
}

func setHookSign(app string, sign string, createTime string) {
	stats := getOrCreateHookStats(app)
	stats.Once = Once{
		Sign:       sign,
		CreateTime: createTime,
	}
}

func updateHookStats(app string, callsInc int32, errorsInc int32) {
	stats := getOrCreateHookStats(app)
	atomic.AddInt32(&stats.Calls, callsInc)
	atomic.AddInt32(&stats.Errors, errorsInc)
	hookStatsMap.Store(app, stats)
}

func addHookCalls(app string, callsInc int32) {
	stats := getOrCreateHookStats(app)
	atomic.AddInt32(&stats.Calls, callsInc)
	hookStatsMap.Store(app, stats)
}

func addHookErrors(app string, errorsInc int32) {
	stats := getOrCreateHookStats(app)
	atomic.AddInt32(&stats.Errors, errorsInc)
	hookStatsMap.Store(app, stats)
}

func resetHookStats(app string) {
	stats := getOrCreateHookStats(app)
	atomic.StoreInt32(&stats.Calls, 0)
	atomic.StoreInt32(&stats.Errors, 0)
}

// func wrappedHookHandler(c *gin.Context) {
// ctx, _ := c.Get("ctx")
// globalCtx := ctx.(context.Context)

// configFilePath := globalCtx.Value("config_file_path").(string)
// hookConfig = getHookConfigFromPath(configFilePath)

// path := c.Request.URL.Path
// appName := getAppName(path)
// log.Printf("request path: %s, app name: %s", path, appName)

// if err := webHookHandler(c, appName); err != nil {
// 	updateHookStats(appName, 0, 1)
// 	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 	return
// }
// }

func webHookHandler(c *gin.Context) {
	var webhookRequest WebhookRequest
	if err := c.ShouldBindJSON(&webhookRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(webhookRequest.EventData.Resources) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no resources found"})
		return
	}

	resourceURL := webhookRequest.EventData.Resources[0].ResourceURL

	appName := getAppName(resourceURL)

	if appName != "" && strings.Contains(resourceURL, "/build-hook/") {
		addHookCalls(appName, 1)
	} else {
		return
	}

	// harbor可能会重试多次
	savedSign := getHookSign(appName)
	currentCreateTime := getTimeFromTag(resourceURL)
	// savedCreatedTime
	currentSign, _ := json.Marshal(webhookRequest.EventData)
	if savedSign.Sign != string(currentSign) {
		if currentCreateTime > savedSign.CreateTime {
			setHookSign(appName, string(currentSign), currentCreateTime)
		} else {
			log.Printf("[ WebHandler ] [ deprecated request ] from %s: %v", appName, resourceURL)
			log.Printf("[ WebHandler ] [ deprecated request ] current sign: %s", currentSign)
			log.Printf("[ WebHandler ] [ deprecated request ] saved sign: %s", savedSign)
			log.Printf("[ WebHandler ] [ deprecated request ] currentCreateTime %s, saved createTime: %s", currentCreateTime, savedSign.CreateTime)
			c.JSON(http.StatusOK, gin.H{"status": "success"})
			return
		}
	} else {
		log.Printf("[ WebHandler ] [ duplicate request ] from %s: %v", appName, resourceURL)
		log.Printf("[ WebHandler ] [ duplicate request ] current sign: %s", currentSign)
		log.Printf("[ WebHandler ] [ duplicate request ] saved sign: %s", savedSign)
		c.JSON(http.StatusOK, gin.H{"status": "success"})
		return
	}

	namespace := webhookRequest.EventData.Repository.Namespace
	// overwrite by appName
	// name := webhookRequest.EventData.Repository.Name
	tag := webhookRequest.EventData.Resources[0].Tag

	mailBodyFile, attachments, err := handlers.ImageHandler(namespace, appName, tag, resourceURL)
	if err != nil {
		addHookErrors(appName, 1)
		c.JSON(http.StatusInternalServerError, gin.H{"Process image error": err.Error()})
		return
	}

	if err := handlers.MailHandler(appName, mailBodyFile, attachments); err != nil {
		addHookErrors(appName, 1)
		c.JSON(http.StatusInternalServerError, gin.H{"Send mail error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func resetStatCounters() {
	var once sync.Once
	resetHookStatsFunc := func() {
		var wg sync.WaitGroup
		hookStatsMap.Range(func(key, value interface{}) bool {
			wg.Add(1)
			hookStats, _ := value.(*HookStats)
			go func(hookStats *HookStats) {
				if err := resetSingleHookState(hookStats); err != nil {
					log.Printf("[ ResetCounter ] Error handling hook stats for %s: %v", hookStats.Name, err)
				}
				wg.Done()
			}(hookStats)
			//清理 stats.json文件
			if err := SaveMapToFile(); err != nil {
				fmt.Printf("Error saving map to file: %v\n", err)
			}
			return true
		})
		wg.Wait()
	}

	for {
		now := time.Now()

		nextResetTime := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		resetDuration := nextResetTime.Sub(now)

		log.Printf("[ ResetCounter ] Current time: %s\n", now.Format(time.RFC3339))
		log.Printf("[ ResetCounter ] Next reset time: %s\n", nextResetTime.Format(time.RFC3339))
		log.Printf("[ ResetCounter ] Waiting %.2f hours and %.2f minutes before reset statistics", math.Floor(resetDuration.Hours()), (resetDuration % time.Hour).Minutes())

		time.Sleep(resetDuration)
		log.Printf("[ ResetCounter ] Reset all stats counter now")
		once.Do(resetHookStatsFunc)
	}

}

func hookStatsInformerFunc() {
	var wg sync.WaitGroup
	jitterTime := time.Duration(rand.Intn(10)) * time.Second
	hookStatsMap.Range(func(key, value interface{}) bool {
		wg.Add(1)
		hookStats, _ := value.(*HookStats)
		go func(hookStats *HookStats) {
			time.Sleep(jitterTime)
			if err := informHookStats(hookStats); err != nil {
				log.Printf("Error informing hook stats for %s: %v", hookStats.Name, err)
			}
			wg.Done()
		}(hookStats)
		return true
	})
	wg.Wait()
}

func informHookStatsByExactTime() {
	var once sync.Once
	config := getHookConfig()

	informTimeList := config.Hook.Audit.InformTime
	sort.Strings(informTimeList)
	for {
		now := time.Now()
		for i, timeStr := range informTimeList {
			criticalTime, err := time.ParseInLocation("15:04", timeStr, time.Local)
			if err != nil {
				log.Fatalf("[ InformByExactTime ] Error parsing time string %s, err: %v", timeStr, err)
			}
			criticalTime = time.Date(now.Year(), now.Month(), now.Day(), criticalTime.Hour(), criticalTime.Minute(), 0, 0, time.Local)

			log.Printf("[ InformByExactTime ] Current time: %s, next inform time: %s\n", now.Format(time.RFC3339), criticalTime.Format(time.RFC3339))

			if now.Before(criticalTime) || now.Equal(criticalTime) {
				waitTime := criticalTime.Sub(now)
				log.Printf("[ InformByExactTime ] Waiting %.2f hours and %.2f minutes before inform trigger", math.Floor(waitTime.Hours()), (waitTime % time.Hour).Minutes())
				time.Sleep(waitTime)
				log.Printf("[ InformByExactTime ] Inform critical time %s has passed, trigger inform", timeStr)
				once.Do(hookStatsInformerFunc)
			} else {
				// 如果当前时间大于最大的时间, 则执行一次
				if i == len(informTimeList)-1 {
					log.Printf("[ InformByExactTime ] inform at least once after %s", timeStr)
					once.Do(hookStatsInformerFunc)
				} else {
					log.Printf("[ InformByExactTime ] ignore validation for %s, wait for next inform time", timeStr)
				}
			}
		}

		now = time.Now()
		nextDayTime := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		waitDuration := nextDayTime.Sub(now)
		log.Printf("[ InformByExactTime ] Waiting %.2f hours and %.2f minutes before new daily routine", math.Floor(waitDuration.Hours()), (waitDuration % time.Hour).Minutes())
		time.Sleep(waitDuration)
	}

}

func informHookStatsByCronExpr() {
	config := getHookConfig()
	cronExpr := config.Hook.Audit.InformCron

	if len(cronExpr) > 0 {
		c := cron.New(cron.WithSeconds()) // 启用秒级精度

		// 解析cron表达式并添加任务
		_, err := c.AddFunc(cronExpr, hookStatsInformerFunc)
		if err != nil {
			fmt.Println("解析cron表达式时出错:", err)
			return
		}

		// 启动cron调度器
		c.Start()
	}
}

func resetSingleHookState(hookStats *HookStats) error {

	// if err := informHookStats(hookStats); err != nil {
	// 	log.Printf("Inform for hook failed %v", err)
	// 	return err
	// }

	// 重置统计计数
	resetHookStats(hookStats.Name)
	log.Printf("Hook stats reset for %s\n", hookStats.Name)
	return nil
}

func informHookStats(hookStats *HookStats) error {
	if hookStats.Calls == 0 {
		log.Printf("No hook calls received today for %s\n", hookStats.Name)
		// 发送没有收到构建的失败邮件
		if err := handlers.SendFailEmail(hookStats.Name); err != nil {
			log.Printf("Failed to send failure email: %v", err)
			return err
		}
	} else if hookStats.Errors > 0 {
		log.Printf("There were %d hook call errors today for %s", hookStats.Errors, hookStats.Name)
		// 发送失败邮件
		if err := handlers.SendWarnEmail(hookStats.Name); err != nil {
			log.Printf("Failed to send warning email: %v", err)
			return err
		}
		// } else {
		// 	// 处理成功, 也发送邮件, 详情邮件在触发的时候已经发过了
		// 	log.Printf("Hook calls successful today for %s\n", hookStats.Name)
		// 	if err := handlers.SendSuccessEmail(hookStats.Name); err != nil {
		// 		log.Printf("Failed to send warning email: %v", err)
		// 		return err
		// 	}
	}
	return nil
}
