package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"net"
	"net/http"
	"path/filepath"
	"sync"
)

var (
	selectedFilePath string
	fileMap          = make(map[string]string) // 用于存储随机字符串和文件路径的映射
	mu               sync.Mutex                // 互斥锁用于保护 map
)

func main() {
	a := app.New()
	w := a.NewWindow("文件传输")
	w.Resize(fyne.NewSize(800, 600))

	// Label：显示已选择的文件
	selectedFile := widget.NewLabel("未选择文件")

	// 按钮：选择文件
	selectButton := widget.NewButton("选择文件", func() {
		dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
			if err == nil && file != nil {
				selectedFilePath = file.URI().Path()
				selectedFile.SetText(selectedFilePath)
			}
		}, w)
	})

	// 多行文本框：展示所有生成的下载链接
	allLinks := widget.NewMultiLineEntry()
	allLinks.SetPlaceHolder("暂无历史记录")
	allLinks.Wrapping = fyne.TextWrapWord

	// 将 allLinks 放入滚动容器中
	scrollableLinks := container.NewVScroll(allLinks)
	scrollableLinks.SetMinSize(fyne.NewSize(800, 300)) // 设置最小高度，确保占据足够空间

	// 按钮：生成 HTTP 连接
	linkEntry := widget.NewEntry()
	startServerButton := widget.NewButton("生成下载链接", func() {
		if selectedFilePath == "" {
			linkEntry.SetText("请先选择文件")
			return
		}

		go startFileServer(selectedFilePath, linkEntry, allLinks)
	})

	w.SetContent(container.NewVBox(
		selectButton,
		selectedFile,
		startServerButton,
		linkEntry,
		widget.NewLabel("已生成的下载链接："),
		scrollableLinks,
	))
	w.ShowAndRun()
}

func startFileServer(filePath string, linkLabel *widget.Entry, allLinks *widget.Entry) {
	// 生成随机字符串作为URL路径
	randomString := generateRandomString(12)

	// 存储文件路径与随机字符串的映射
	mu.Lock()
	fileMap[randomString] = filePath
	mu.Unlock()

	// 动态生成路由 /send-file/randomString
	http.HandleFunc("/send-file/"+randomString, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		file := fileMap[randomString]
		mu.Unlock()

		fileName := filepath.Base(file)
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		http.ServeFile(w, r, file)
	})

	// 获取本地 IP 地址
	ip := getLocalIP()
	port := "32123" // 固定端口
	downloadURL := fmt.Sprintf("http://%s:%s/send-file/%s", ip, port, randomString)
	linkLabel.SetText(downloadURL)

	// 更新所有生成的链接信息
	updateAllLinks(allLinks)

	fmt.Println("服务器启动, 文件地址:", downloadURL)
	_ = http.ListenAndServe(":"+port, nil)
}

// generateRandomString 生成指定长度的随机字符串
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}

	for _, address := range addrs {
		// 检查 IP 地址类型，并且排除回环地址
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "localhost"
}

// updateAllLinks 更新并展示所有已生成的下载链接
func updateAllLinks(allLinks *widget.Entry) {
	mu.Lock()
	defer mu.Unlock()

	var linksText string
	for key, filePath := range fileMap {
		linksText += fmt.Sprintf("文件: %s\n链接: http://%s:%s/send-file/%s\n\n", filepath.Base(filePath), getLocalIP(), "32123", key)
	}

	allLinks.SetText(linksText)
}
