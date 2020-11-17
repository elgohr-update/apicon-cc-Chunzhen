package main

import (
	"net"

	"github.com/gin-gonic/gin"
	"github.com/wuhan005/gadget"
	log "unknwon.dev/clog/v2"
)

func init() {
	_ = log.NewConsole(100)
}

func main() {
	db := NewQQwry("./qqwry/qqwry.dat")

	r := gin.Default()

	r.NoRoute(func(c *gin.Context) {
		c.JSON(gadget.MakeErrJSON(404, 40400, ""))
	})

	r.GET("/", func(c *gin.Context) {
		ipStr, ok := c.GetQuery("ip")
		if !ok {
			ipStr = c.ClientIP()
		}

		ip := net.ParseIP(ipStr)
		if ip == nil {
			c.JSON(gadget.MakeErrJSON(400, 40000, "Error ip input."))
			return
		}

		country, area := db.Find(ip.String())

		c.JSON(gadget.MakeSuccessJSON(gin.H{
			"country": country,
			"area":    area,
			"ip":      ip.String(),
		}))
	})

	_ = r.Run(":8080")
}
