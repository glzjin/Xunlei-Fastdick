package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Config struct {
	XUNLEI_UID    string `json:"xunlei_uid"`
	XUNLEI_PASSWD string `json:"xunlei_passwd"`
}

type DisplayConfig struct {
	XUNLEI_UID    string
	XUNLEI_PASSWD string
}

// 全局变量
var (
	currentProcess *exec.Cmd
	processLock    sync.Mutex
	goLogFile      *os.File // Go程序的日志文件
	pythonLogFile  *os.File // Python进程的输出日志文件
	goLogWriter    io.Writer
)

// 初始化日志
func initLog() error {
	var err error
	// 初始化Go程序的日志文件
	goLogFile, err = os.OpenFile("/data/swjsq.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开Go日志文件失败: %v", err)
	}
	goLogWriter = io.MultiWriter(goLogFile, os.Stdout)

	// 初始化Python进程的日志文件
	pythonLogFile, err = os.OpenFile("/data/swjsq2.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		goLogFile.Close()
		return fmt.Errorf("打开Python日志文件失败: %v", err)
	}

	// 设置标准日志输出
	log.SetOutput(goLogWriter)
	return nil
}

// 写入日志的辅助函数
func writeLog(format string, v ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, v...)
	fmt.Fprintf(goLogWriter, "%s %s\n", timestamp, msg)
}

// 启动Python进程
func startPythonProcess(uid, passwd string) error {
	processLock.Lock()
	defer processLock.Unlock()

	// 如果已有进程在运行，先用 SIGKILL 结束它
	if currentProcess != nil && currentProcess.Process != nil {
		writeLog("正在终止现有Python进程")
		if err := currentProcess.Process.Kill(); err != nil {
			writeLog("终止进程出错: %v", err)
		}
		currentProcess.Wait()
		exec.Command("pkill", "-9", "-f", "swjsq.py").Run()
	}

	// 删除会话文件
	sessionFile := "/app/.swjsq.session"
	if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
		writeLog("删除会话文件失败: %v", err)
		// 继续执行，不要因为删除失败而中断
	}

	writeLog("正在启动Python进程，UID=%s", uid)

	// 准备环境变量
	env := append(os.Environ(),
		fmt.Sprintf("XUNLEI_UID=%s", uid),
		fmt.Sprintf("XUNLEI_PASSWD=%s", passwd),
	)

	// 创建新进程
	cmd := exec.Command("python", "/app/swjsq.py")
	cmd.Env = env

	// 创建多重写入器，同时写入Python日志文件和标准输出
	pythonWriter := io.MultiWriter(pythonLogFile, os.Stdout)
	cmd.Stdout = pythonWriter
	cmd.Stderr = pythonWriter

	// 启动进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动Python进程失败: %v", err)
	}

	currentProcess = cmd

	// 在后台等待进程结束
	go func() {
		err := cmd.Wait()
		if err != nil {
			writeLog("Python进程退出: %v", err)
		}
	}()

	return nil
}

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>迅雷快鸟配置</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; }
        input[type="text"], input[type="password"] { width: 100%; padding: 8px; }
        button { padding: 10px 20px; background-color: #4CAF50; color: white; border: none; cursor: pointer; }
        button:hover { background-color: #45a049; }
        #log-container { 
            margin-top: 20px;
            padding: 10px;
            background: #f5f5f5;
            border: 1px solid #ddd;
            border-radius: 4px;
            height: 300px;
            overflow-y: auto;
            font-family: monospace;
            white-space: pre-wrap;
        }
        .refresh-btn {
            margin-top: 10px;
            background-color: #2196F3;
        }
        .refresh-btn:hover {
            background-color: #1976D2;
        }
    </style>
    <script>
        function refreshLogs() {
            fetch('/logs')
                .then(response => response.text())
                .then(data => {
                    document.getElementById('log-container').textContent = data;
                    document.getElementById('log-container').scrollTop = document.getElementById('log-container').scrollHeight;
                });
        }

        window.onload = function() {
            refreshLogs();
            setInterval(refreshLogs, 10000);
        }
    </script>
</head>
<body>
    <h2>迅雷快鸟配置</h2>
    <form method="POST" action="/save">
        <div class="form-group">
            <label for="uid">迅雷UID:</label>
            <input type="text" id="uid" name="uid" value="{{.XUNLEI_UID}}">
        </div>
        <div class="form-group">
            <label for="passwd">迅雷密码:</label>
            <input type="password" id="passwd" name="passwd" value="{{.XUNLEI_PASSWD}}" placeholder="不修改密码请保持为空">
        </div>
        <button type="submit">保存配置</button>
    </form>

    <h3>运行日志</h3>
    <button onclick="refreshLogs()" class="refresh-btn">刷新日志</button>
    <div id="log-container"></div>
</body>
</html>
`

func loadConfig() DisplayConfig {
	config := Config{}
	data, err := os.ReadFile("/data/xunlei.json")
	if err == nil {
		json.Unmarshal(data, &config)
	}
	return DisplayConfig{
		XUNLEI_UID:    config.XUNLEI_UID,
		XUNLEI_PASSWD: "****",
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("index").Parse(htmlTemplate))
	config := loadConfig()
	tmpl.Execute(w, config)
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeLog("收到配置保存请求")
	// 读取现有配置
	existingConfig := Config{}
	data, err := os.ReadFile("/data/xunlei.json")
	if err == nil {
		json.Unmarshal(data, &existingConfig)
	}

	// 获取新的配置
	newPasswd := r.FormValue("passwd")
	config := Config{
		XUNLEI_UID:    r.FormValue("uid"),
		XUNLEI_PASSWD: existingConfig.XUNLEI_PASSWD,
	}

	// 只有当用户输入了新密码时才更新密码
	if newPasswd != "" && newPasswd != "****" {
		config.XUNLEI_PASSWD = newPasswd
	}

	// 保存配置
	data, err = json.MarshalIndent(config, "", "    ")
	if err != nil {
		http.Error(w, "Error creating JSON", http.StatusInternalServerError)
		return
	}

	err = os.WriteFile("/data/xunlei.json", data, 0644)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	// 每次保存配置都重启Python进程
	if err := startPythonProcess(config.XUNLEI_UID, config.XUNLEI_PASSWD); err != nil {
		writeLog("Error starting Python process: %v", err)
		http.Error(w, "配置已保存，但启动进程失败", http.StatusInternalServerError)
		return
	}

	writeLog("配置保存成功，进程已重启")

	w.Write([]byte(`
    <html>
        <body>
            <h3>保存成功！配置已生效</h3>
            <a href="/">返回首页</a>
        </body>
    </html>
    `))
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	// 读取两个日志文件
	goLogs, err1 := os.ReadFile("/data/swjsq.log")
	pythonLogs, err2 := os.ReadFile("/data/swjsq2.log")

	var allLogs []string

	// 处理Go程序日志
	if err1 == nil {
		allLogs = append(allLogs, strings.Split(string(goLogs), "\n")...)
	}

	// 处理Python进程日志
	if err2 == nil {
		allLogs = append(allLogs, strings.Split(string(pythonLogs), "\n")...)
	}

	// 如果两个日志文件都读取失败
	if err1 != nil && err2 != nil {
		if os.IsNotExist(err1) && os.IsNotExist(err2) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			return
		}
		http.Error(w, "无法读取日志文件", http.StatusInternalServerError)
		return
	}

	// 过滤空行
	var filteredLogs []string
	for _, line := range allLogs {
		if line != "" {
			filteredLogs = append(filteredLogs, line)
		}
	}

	// 只保留最后1000行
	if len(filteredLogs) > 1000 {
		filteredLogs = filteredLogs[len(filteredLogs)-1000:]
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, strings.Join(filteredLogs, "\n"))
}

func main() {
	// 初始化日志
	if err := initLog(); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer goLogFile.Close()
	defer pythonLogFile.Close()

	writeLog("Web服务器启动")

	// 启动时读取配置并启动Python进程
	config := Config{}
	if data, err := os.ReadFile("/data/xunlei.json"); err == nil {
		if err := json.Unmarshal(data, &config); err == nil {
			writeLog("从配置文件加载账号信息")
			if err := startPythonProcess(config.XUNLEI_UID, config.XUNLEI_PASSWD); err != nil {
				writeLog("启动初始Python进程失败: %v", err)
			}
		}
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/save", handleSave)
	http.HandleFunc("/logs", handleLogs)

	writeLog("开始监听端口 :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		writeLog("服务器错误: %v", err)
		log.Fatal(err)
	}
}
