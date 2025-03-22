package utils

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"multivator/src/config"
	"multivator/src/types"
)

func InitLogger() {
	logFile, err := os.OpenFile(fmt.Sprintf("node%d.log", config.NodeID), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		panic(err)
	}
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	os.Stdout = logFile
	os.Stderr = logFile
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		Level:     config.LogLevel,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("15:04:05"))
				}
			}
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					file := source.File
					if lastSlash := strings.LastIndexByte(file, '/'); lastSlash >= 0 {
						file = file[lastSlash+1:]
					}
					a.Value = slog.StringValue(fmt.Sprintf("%s:%d", file, source.Line))
				}
			}
			return a
		},
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func ForEachOrder(orders types.Orders, action func(node, floor, btn int)) {
	for node := range orders {
		for floor := range orders[node] {
			for btn := range orders[node][floor] {
				action(node, floor, btn)
			}
		}
	}
}
