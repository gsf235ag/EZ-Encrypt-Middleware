package main

import (
	"EZ-Encrypt-Middleware/config"
	"EZ-Encrypt-Middleware/proxy"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	config.LoadConfig()

	// Set Gin mode
	if config.AppConfig.DebugMode != "true" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin engine
	r := gin.Default()

	r.NoRoute(func(c *gin.Context) {
		if config.AppConfig.IsPaymentNotifyPath(c.Request.URL.Path) {
			handlePaymentNotify(c)
			return
		}

		proxy.ProxyHandler(c)
	})

	corsConfig := cors.Config{}

	if config.AppConfig.CORSOrigin == "*" {
		corsConfig.AllowAllOrigins = true
	} else {
		corsConfig.AllowOrigins = config.AppConfig.GetAllowedOrigins()
	}

	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Accept", "Authorization", "x-iv"}
	corsConfig.AllowCredentials = true

	r.Use(cors.New(corsConfig))

	timeout := 30 * time.Second
	if config.AppConfig.RequestTimeout != "" {
		if t, err := time.ParseDuration(config.AppConfig.RequestTimeout + "ms"); err == nil {
			timeout = t
		}
	}

	if config.AppConfig.EnableLogging == "true" {
		r.Use(func(c *gin.Context) {
			log.Printf("请求: %s %s", c.Request.Method, c.Request.URL.Path)

			c.Next()

			status := c.Writer.Status()
			if status >= 400 {
				log.Printf("错误请求: %s %s Status: %d", c.Request.Method, c.Request.URL.Path, status)
			} else {
				log.Printf("响应: %s %s Status: %d", c.Request.Method, c.Request.URL.Path, status)
			}
		})
	}

	port := config.AppConfig.Port
	if port == "" {
		port = "3000"
	}

	log.Printf("服务器运行在端口 %s", port)
	log.Printf("请求超时设置: %v", timeout)
	log.Printf("CORS配置 - 允许所有来源: %t", corsConfig.AllowAllOrigins)
	if !corsConfig.AllowAllOrigins && len(corsConfig.AllowOrigins) > 0 {
		log.Printf("允许的来源: %v", corsConfig.AllowOrigins)
	}
	log.Printf("调试模式: %s", config.AppConfig.DebugMode)
	log.Printf("日志记录: %s", config.AppConfig.EnableLogging)
	if len(config.AppConfig.GetAllowedPaymentNotifyPaths()) > 0 {
		log.Printf("支付回调路径: %v", config.AppConfig.GetAllowedPaymentNotifyPaths())
	}
	r.Run(":" + port)
}

func handlePaymentNotify(c *gin.Context) {
	backendURL := config.AppConfig.BackendAPIURL

	targetURL := backendURL + c.Request.URL.Path

	target, err := url.Parse(targetURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无效的目标URL"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		req.URL.RawQuery = target.RawQuery
		req.Host = target.Host
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
