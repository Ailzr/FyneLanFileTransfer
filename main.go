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
	"strings"
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

	// 主页处理函数
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, "<html><head><title>文件下载</title></head><body>")
		fmt.Fprintln(w, "<h2>可用下载文件</h2><ul>")
		if len(fileMap) == 0 {
			fmt.Fprintf(w, "<p>暂无可下载文件</p>")
		} else {
			for key, filePath := range fileMap {
				downloadURL := fmt.Sprintf("http://%s:%s/send-file/%s", getConnectedPhysicalIP(), "32123", key)
				fmt.Fprintf(w, "<li><a href='%s'>%s</a></li>", downloadURL, filepath.Base(filePath))
			}
		}
		fmt.Fprintln(w, "</ul></body></html>")
	})

	serviceUrl := "http://" + getConnectedPhysicalIP() + ":32123/"
	serviceEntry := widget.NewEntry()
	serviceEntry.SetText(serviceUrl)

	w.SetContent(container.NewVBox(
		selectButton,
		selectedFile,
		startServerButton,
		linkEntry,
		widget.NewLabel("已生成的下载链接：\n可通过总览下载该链接总览下载:"),
		serviceEntry,
		scrollableLinks,
	))
	go func() {
		_ = http.ListenAndServe(":32123", nil)
	}()
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
	ip := getConnectedPhysicalIP()
	port := "32123" // 固定端口
	downloadURL := fmt.Sprintf("http://%s:%s/send-file/%s", ip, port, randomString)
	linkLabel.SetText(downloadURL)

	// 更新所有生成的链接信息
	updateAllLinks(allLinks)

	//fmt.Println("服务器启动, 文件地址:", downloadURL)
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

func getConnectedPhysicalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("无法获取网络接口:", err)
		return "localhost"
	}

	for _, iface := range interfaces {
		// 排除未启用、回环接口或没有有效硬件地址的接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
			continue
		}

		// 过滤常见的虚拟网卡或 VPN 接口的硬件地址前缀
		if isVirtualOrVPN(iface.HardwareAddr.String()) {
			continue
		}

		// 过滤名字中包含VPN字段的网卡
		if strings.Contains(iface.Name, "VPN") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				// 仅返回 IPv4 地址
				if ip := ipNet.IP.To4(); ip != nil {
					return ip.String()
				}
			}
		}
	}
	return "localhost" // 无有效连接时返回 localhost
}

// 判断硬件地址是否属于虚拟网卡或 VPN
func isVirtualOrVPN(macAddr string) bool {
	// 常见虚拟网卡和 VPN 的 MAC 地址前缀（可以根据实际情况扩展或修改）
	virtualPrefixes := []string{
		"00:05:69", // VMware
		"00:0C:29", // VMware
		"00:50:56", // VMware
		"08:00:27", // VirtualBox
		"52:54:00", // QEMU, KVM
		"00:1C:42", // Parallels
		"00:16:3E", // Xen
		"00:1D:D8", // Microsoft Hyper-V
	}

	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(macAddr, prefix) {
			return true
		}
	}
	return false
}

// updateAllLinks 更新并展示所有已生成的下载链接
func updateAllLinks(allLinks *widget.Entry) {
	mu.Lock()
	defer mu.Unlock()

	var linksText string
	for key, filePath := range fileMap {
		linksText += fmt.Sprintf("文件: %s\n链接: http://%s:%s/send-file/%s\n\n", filepath.Base(filePath), getConnectedPhysicalIP(), "32123", key)
	}

	allLinks.SetText(linksText)
}
