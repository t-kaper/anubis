package common

import (
	"github.com/gin-gonic/gin"
	"net"
	"strings"
)

func GetClientIP(ctx *gin.Context) string {
	ip := ctx.GetHeader("X-Real-IP")
	if ip == "" {
		ip = ctx.GetHeader("X-Forwarded-For")
	}

	if ip != "" {
		ips := strings.Split(ip, ",")
		ip = strings.TrimSpace(ips[0])
	}

	if ip == "" {
		ip, _, _ = net.SplitHostPort(ctx.Request.RemoteAddr)
	}

	return ip
}

func GetDeviceType(ctx *gin.Context) string {
	// Prefer explicit client-sent device marker (mobile/web/etc).
	deviceType := strings.TrimSpace(ctx.GetHeader("X-Device"))
	if deviceType == "" {
		deviceType = strings.TrimSpace(ctx.Request.UserAgent())
	}
	if deviceType == "" {
		return "_"
	}
	// Prevent unbounded storage/logging.
	const maxLen = 256
	if len(deviceType) > maxLen {
		deviceType = deviceType[:maxLen]
	}
	return deviceType
}
