package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/exyb/harbor-hook-to-mail/config"
	"github.com/exyb/harbor-hook-to-mail/handlers"
	"github.com/robfig/cron/v3"

	"github.com/gin-gonic/gin"
)

type Once struct {
	Sign       string
	createTime int64
}

type HookStats struct {
	Name   string
	Calls  int32
	Errors int32
	Once   Once
}

var (
	hookStatsMap sync.Map
	hookConfig   *HookConfig
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

	r.POST(hookConfig.Hook.ContextPath, webHookHandler)
	// hookGroup := r.Group("/hook")
	// {
	// 	hookGroup.POST("/:app", wrappedHookHandler)
	// }

	go resetStatCounters()
	go informHookStatsByCronExpr()
}

func getAppName(path string) string {
	parts := strings.Split(strings.Split(path, ":")[0], "/")
	if len(parts) < 3 {
		return ""
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
	if !ok {
		stats = &HookStats{
			Name:   app,
			Calls:  0,
			Errors: 0,
			Once: Once{
				Sign:       "",
				createTime: 0,
			},
		}
		_, _ = hookStatsMap.LoadOrStore(app, stats)
		return stats
	}
	stats, _ = value.(*HookStats)
	return stats
}

func getHookSign(app string) Once {
	stats := getOrCreateHookStats(app)
	return stats.Once
}

func setHookSign(app string, sign string, createTime int64) {
	stats := getOrCreateHookStats(app)
	stats.Once = Once{
		Sign:       sign,
		createTime: createTime,
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
	currentCreateTime := webhookRequest.EventData.Repository.DateCreated
	currentSign, _ := json.Marshal(webhookRequest.EventData.Resources)
	if savedSign.Sign != string(currentSign) {
		if currentCreateTime > savedSign.createTime {
			setHookSign(appName, string(currentSign), currentCreateTime)
		} else {
			log.Printf("deprecated request from %s: %v", appName, resourceURL)
			c.JSON(http.StatusOK, gin.H{"status": "success"})
			return
		}
	} else {
		log.Printf("duplicate request from %s: %v", appName, resourceURL)
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
	config := getHookConfig()
	resetHookStatsFunc := func() {
		var wg sync.WaitGroup
		hookStatsMap.Range(func(key, value interface{}) bool {
			wg.Add(1)
			hookStats, _ := value.(*HookStats)
			go func(hookStats *HookStats) {
				if err := resetSingleHookState(hookStats); err != nil {
					log.Printf("Error handling hook stats for %s: %v", hookStats.Name, err)
				}
				wg.Done()
			}(hookStats)
			return true
		})
		wg.Wait()
	}

	for {
		now := time.Now()
		criticalTime, err := time.ParseInLocation("15:04", config.Hook.Audit.Before, time.Local)
		if err != nil {
			log.Fatalf("error parsing config.Hook.Audit.Before: %v", err)
		}
		criticalTime = time.Date(now.Year(), now.Month(), now.Day(), criticalTime.Hour(), criticalTime.Minute(), 0, 0, time.Local)

		nextResetTime := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		resetDuration := nextResetTime.Sub(now)

		log.Printf("Current time: %s\n", now.Format(time.RFC3339))
		log.Printf("To time: %s\n", criticalTime.Format(time.RFC3339))
		log.Printf("Next reset time: %s\n", nextResetTime.Format(time.RFC3339))
		log.Printf("Waiting %.2f hours and %.2f minutes before reset statistics", math.Round(resetDuration.Hours()), (resetDuration % time.Hour).Minutes())

		if now.Before(criticalTime) {
			waitTime := criticalTime.Sub(now)
			log.Printf("Waiting %.2f hours and %.2f minutes before validation trigger", math.Round(waitTime.Hours()), (waitTime % time.Hour).Minutes())

			time.Sleep(waitTime)
		}
		time.Sleep(time.Minute * 1)

		if now.After(criticalTime) {
			log.Printf("Validation critical time has passed, trigger validation now")
			once.Do(resetHookStatsFunc)
		}

		time.Sleep(resetDuration)
	}

}

func informHookStatsByCronExpr() {
	config := getHookConfig()
	cronExpr := config.Hook.Audit.InformCron
	hookStatsInformerFunc := func() {
		var wg sync.WaitGroup
		hookStatsMap.Range(func(key, value interface{}) bool {
			wg.Add(1)
			hookStats, _ := value.(*HookStats)
			go func(hookStats *HookStats) {
				if err := informHookStats(hookStats); err != nil {
					log.Printf("Error informing hook stats for %s: %v", hookStats.Name, err)
				}
				wg.Done()
			}(hookStats)
			return true
		})
		wg.Wait()
	}
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

func resetSingleHookState(hookStats *HookStats) error {

	if err := informHookStats(hookStats); err != nil {
		log.Printf("Inform for hook failed %v", err)
		return err
	}

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
	} else {
		// 处理成功, 也发送邮件, 详情邮件在触发的时候已经发过了
		log.Printf("Hook calls successful today for %s\n", hookStats.Name)
		if err := handlers.SendSuccessEmail(hookStats.Name); err != nil {
			log.Printf("Failed to send warning email: %v", err)
			return err
		}
	}
	return nil
}
