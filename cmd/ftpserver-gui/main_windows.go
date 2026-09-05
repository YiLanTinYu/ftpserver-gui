//go:build windows

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fclairamb/ftpserver/config"
	"github.com/fclairamb/ftpserver/config/confpar"
	serverdriver "github.com/fclairamb/ftpserver/server"
	ftpserverlib "github.com/fclairamb/ftpserverlib"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows/registry"
)

const (
	appName             = "FTP绿色版服务端"
	appVersion          = "0.7.1"
	startupValueName    = "FTP绿色版服务端"
	startupRegistryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
)

//go:embed ftpserver-logo.png
var embeddedLogoPNG []byte

type app struct {
	mw           *walk.MainWindow
	appIcon      *walk.Icon
	notifyIcon   *walk.NotifyIcon
	statusLabel  *walk.Label
	addressLabel *walk.Label
	logView      *walk.TextEdit
	listenIP     *walk.ComboBox
	listenItems  []string
	listenIPs    map[string]string
	ftpPort      *walk.NumberEdit
	pasvStart    *walk.NumberEdit
	pasvEnd      *walk.NumberEdit
	maxClients   *walk.NumberEdit
	idleTimeout  *walk.NumberEdit
	userList     *walk.ListBox
	username     *walk.LineEdit
	password     *walk.LineEdit
	showPassword *walk.CheckBox
	rootDir      *walk.LineEdit
	readOnly     *walk.CheckBox
	startButton  *walk.PushButton
	stopButton   *walk.PushButton
	startupCheck *walk.CheckBox
	startupReady bool
	users        []*confpar.Access
	content      *confpar.Content
	configPath   string
	server       *ftpserverlib.FtpServer
	driver       *serverdriver.Server
	running      bool
	allowExit    bool
	mu           sync.Mutex
}

func main() {
	exe, err := os.Executable()
	if err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}

	a := &app{configPath: "ftpserver.json"}
	a.loadListenAddresses()
	if err := a.loadConfig(); err != nil {
		walk.MsgBox(nil, appName, "读取配置失败，将使用默认设置：\r\n"+err.Error(), walk.MsgBoxIconWarning)
		a.content = defaultContent()
		a.users = a.content.Accesses
	}

	if err := a.buildWindow(); err != nil {
		walk.MsgBox(nil, appName, err.Error(), walk.MsgBoxIconError)
		return
	}
	if err := a.setupTray(); err != nil {
		walk.MsgBox(a.mw, appName, "系统托盘初始化失败：\r\n"+err.Error(), walk.MsgBoxIconWarning)
	}
	a.populateForm()
	a.loadStartupSetting()
	a.appendLog("程序已启动，版本 " + appVersion)
	a.mw.Run()
}

func defaultContent() *confpar.Content {
	return &confpar.Content{
		Version:                  1,
		ListenAddress:            net.JoinHostPort(preferredLocalIPv4(), "12121"),
		MaxClients:               10,
		IdleTimeout:              duration(15 * time.Minute),
		PassiveTransferPortRange: &confpar.PortRange{Start: 2122, End: 2130},
		Logging:                  confpar.Logging{File: filepath.Join("logs", "ftpserver.log")},
		Accesses:                 []*confpar.Access{},
	}
}

func duration(v time.Duration) confpar.Duration { return confpar.Duration{Duration: v} }

func (a *app) loadConfig() error {
	b, err := os.ReadFile(a.configPath)
	if errors.Is(err, os.ErrNotExist) {
		a.content = defaultContent()
		a.users = a.content.Accesses
		return nil
	}
	if err != nil {
		return err
	}
	var c confpar.Content
	if err := json.Unmarshal(b, &c); err != nil {
		return err
	}
	if c.ListenAddress == "" {
		c.ListenAddress = net.JoinHostPort(preferredLocalIPv4(), "12121")
	}
	if c.PassiveTransferPortRange == nil {
		c.PassiveTransferPortRange = &confpar.PortRange{Start: 2122, End: 2130}
	}
	if c.Logging.File == "" {
		c.Logging.File = filepath.Join("logs", "ftpserver.log")
	}
	a.content = &c
	a.users = c.Accesses
	return nil
}

func (a *app) buildWindow() error {
	var err error
	if a.appIcon, err = embeddedLogoIcon(); err != nil {
		return fmt.Errorf("加载软件图标失败: %w", err)
	}

	return MainWindow{
		AssignTo: &a.mw,
		Title:    appName + "  v" + appVersion,
		Icon:     a.appIcon,
		MinSize:  Size{Width: 660, Height: 540},
		Size:     Size{Width: 720, Height: 590},
		Layout:   VBox{MarginsZero: false},
		Children: []Widget{
			Composite{Layout: HBox{}, Children: []Widget{
				Label{AssignTo: &a.statusLabel, Text: "● 服务未启动"},
				HSpacer{},
				Label{Text: "Make by 倚栏听雨    Version " + appVersion},
			}},
			GroupBox{Title: "服务器设置", Layout: Grid{Columns: 4}, Children: []Widget{
				Label{Text: "监听IP："}, ComboBox{AssignTo: &a.listenIP, Model: a.listenItems, Editable: true, ColumnSpan: 3},
				Label{Text: "FTP端口："}, NumberEdit{AssignTo: &a.ftpPort, MinValue: 1, MaxValue: 65535, Decimals: 0},
				Label{Text: "最大连接："}, NumberEdit{AssignTo: &a.maxClients, MinValue: 0, MaxValue: 100000, Decimals: 0},
				Label{Text: "被动端口："}, NumberEdit{AssignTo: &a.pasvStart, MinValue: 1, MaxValue: 65535, Decimals: 0},
				Label{Text: "至"}, NumberEdit{AssignTo: &a.pasvEnd, MinValue: 1, MaxValue: 65535, Decimals: 0},
				Label{Text: "空闲超时："}, NumberEdit{AssignTo: &a.idleTimeout, MinValue: 0, MaxValue: 100000, Decimals: 0},
				Label{Text: "分钟"}, PushButton{Text: "高级设置...", OnClicked: a.showAdvanced},
			}},
			GroupBox{Title: "FTP账户与访问目录", Layout: HBox{}, Children: []Widget{
				Composite{Layout: VBox{}, Children: []Widget{
					Label{Text: "账户列表"},
					ListBox{AssignTo: &a.userList, MinSize: Size{Width: 180, Height: 125}, OnCurrentIndexChanged: a.selectUser},
					Composite{Layout: HBox{}, Children: []Widget{
						PushButton{Text: "新增账户", OnClicked: a.newUser},
						PushButton{Text: "删除", OnClicked: a.deleteUser},
					}},
				}},
				Composite{Layout: Grid{Columns: 3}, Children: []Widget{
					Label{Text: "账户名称："}, LineEdit{AssignTo: &a.username, ColumnSpan: 2},
					Label{Text: "账户密码："}, LineEdit{AssignTo: &a.password, PasswordMode: true}, Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						PushButton{Text: "随机生成", OnClicked: a.randomPassword},
						PushButton{Text: "复制密码", OnClicked: a.copyPassword},
					}},
					Label{Text: "密码显示："}, CheckBox{AssignTo: &a.showPassword, Text: "显示密码", OnCheckedChanged: a.changePasswordVisibility, ColumnSpan: 2},
					Label{Text: "访问目录："}, LineEdit{AssignTo: &a.rootDir}, PushButton{Text: "选择目录...", OnClicked: a.browseRoot},
					Label{Text: "账户权限："}, CheckBox{AssignTo: &a.readOnly, Text: "只允许下载（只读）", ColumnSpan: 2},
					HSpacer{}, PushButton{Text: "保存账户", OnClicked: a.saveCurrentUser, ColumnSpan: 2},
				}},
			}},
			GroupBox{Title: "服务信息", Layout: VBox{}, Children: []Widget{
				Label{AssignTo: &a.addressLabel, Text: "连接地址：尚未启动"},
				TextEdit{AssignTo: &a.logView, ReadOnly: true, VScroll: true, MinSize: Size{Height: 75}},
			}},
			Composite{Layout: HBox{}, Children: []Widget{
				PushButton{AssignTo: &a.startButton, Text: "启动服务", OnClicked: a.start},
				PushButton{AssignTo: &a.stopButton, Text: "停止服务", Enabled: false, OnClicked: a.stop},
				PushButton{Text: "保存设置", OnClicked: func() { a.saveConfig() }},
				CheckBox{AssignTo: &a.startupCheck, Text: "开机启动", OnCheckedChanged: a.changeStartup},
				HSpacer{},
				PushButton{Text: "关于", OnClicked: a.about},
				PushButton{Text: "退出", OnClicked: a.exit},
			}},
		},
	}.Create()
}

func (a *app) setupTray() error {
	a.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if !a.allowExit {
			*canceled = true
			a.mw.Hide()
			if a.notifyIcon != nil {
				_ = a.notifyIcon.ShowMessage(appName, "程序仍在后台运行，可从系统托盘恢复窗口。")
			}
		}
	})

	ni, err := walk.NewNotifyIcon(a.mw)
	if err != nil {
		return err
	}
	a.notifyIcon = ni
	if err := ni.SetIcon(a.appIcon); err != nil {
		return err
	}
	if err := ni.SetToolTip(appName); err != nil {
		return err
	}

	showAction := walk.NewAction()
	showAction.SetText("显示主窗口")
	showAction.Triggered().Attach(func() {
		a.mw.Show()
		_ = a.mw.SetFocus()
	})
	startAction := walk.NewAction()
	startAction.SetText("启动服务")
	startAction.Triggered().Attach(a.start)
	stopAction := walk.NewAction()
	stopAction.SetText("停止服务")
	stopAction.Triggered().Attach(a.stop)
	exitAction := walk.NewAction()
	exitAction.SetText("退出")
	exitAction.Triggered().Attach(a.exit)
	_ = ni.ContextMenu().Actions().Add(showAction)
	_ = ni.ContextMenu().Actions().Add(startAction)
	_ = ni.ContextMenu().Actions().Add(stopAction)
	_ = ni.ContextMenu().Actions().Add(exitAction)
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.mw.Show()
			_ = a.mw.SetFocus()
		}
	})
	return ni.SetVisible(true)
}

func (a *app) populateForm() {
	host, port, err := net.SplitHostPort(a.content.ListenAddress)
	if err != nil {
		host, port = "0.0.0.0", "2121"
	}
	a.listenIP.SetText(a.listenDisplay(host))
	p, _ := strconv.Atoi(port)
	a.ftpPort.SetValue(float64(p))
	a.pasvStart.SetValue(float64(a.content.PassiveTransferPortRange.Start))
	a.pasvEnd.SetValue(float64(a.content.PassiveTransferPortRange.End))
	a.maxClients.SetValue(float64(a.content.MaxClients))
	a.idleTimeout.SetValue(a.content.IdleTimeout.Minutes())
	a.refreshUsers()
}

func (a *app) loadStartupSetting() {
	enabled, err := startupEnabled()
	a.startupCheck.SetChecked(enabled)
	a.startupReady = true
	if err != nil {
		a.appendLog("读取开机启动设置失败：" + err.Error())
	}
}

func (a *app) changeStartup() {
	if !a.startupReady {
		return
	}
	enabled := a.startupCheck.Checked()
	if err := setStartupEnabled(enabled); err != nil {
		a.startupReady = false
		a.startupCheck.SetChecked(!enabled)
		a.startupReady = true
		walk.MsgBox(a.mw, "开机启动", "修改开机启动设置失败：\r\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	if enabled {
		a.appendLog("已启用当前用户开机启动")
	} else {
		a.appendLog("已关闭当前用户开机启动")
	}
}

func startupEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, startupRegistryPath, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer key.Close()
	value, _, err := key.GetStringValue(startupValueName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	exePath, err := os.Executable()
	if err != nil {
		return false, err
	}
	return value == startupCommand(exePath), nil
}

func setStartupEnabled(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, startupRegistryPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if !enabled {
		if err := key.DeleteValue(startupValueName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	return key.SetStringValue(startupValueName, startupCommand(exePath))
}

func (a *app) collectForm() error {
	listenIP := a.selectedListenIP()
	if listenIP == "" {
		return errors.New("监听IP不能为空")
	}
	if net.ParseIP(listenIP) == nil {
		return errors.New("监听IP必须是有效IP地址")
	}
	port := int(a.ftpPort.Value())
	start, end := int(a.pasvStart.Value()), int(a.pasvEnd.Value())
	if start > end {
		return errors.New("被动端口起始值不能大于结束值")
	}
	if port >= start && port <= end {
		return errors.New("FTP端口不能位于被动端口范围内")
	}
	if len(a.users) == 0 {
		return errors.New("至少需要添加一个FTP用户")
	}
	for _, u := range a.users {
		if strings.TrimSpace(u.User) == "" || u.Pass == "" || u.Params["basePath"] == "" {
			return errors.New("每个用户都必须设置用户名、密码和共享目录")
		}
		if st, err := os.Stat(u.Params["basePath"]); err != nil || !st.IsDir() {
			return fmt.Errorf("用户 %s 的共享目录不存在", u.User)
		}
	}
	a.content.Version = 1
	a.content.ListenAddress = net.JoinHostPort(listenIP, strconv.Itoa(port))
	a.content.MaxClients = int(a.maxClients.Value())
	a.content.IdleTimeout = duration(time.Duration(a.idleTimeout.Value()) * time.Minute)
	a.content.PassiveTransferPortRange = &confpar.PortRange{Start: start, End: end}
	a.content.Accesses = a.users
	return nil
}

func embeddedLogoIcon() (*walk.Icon, error) {
	logoImage, err := png.Decode(bytes.NewReader(embeddedLogoPNG))
	if err != nil {
		return nil, fmt.Errorf("解析内嵌图标失败: %w", err)
	}
	return walk.NewIconFromImageForDPI(logoImage, 192)
}

func (a *app) saveConfig() bool {
	if err := a.collectForm(); err != nil {
		walk.MsgBox(a.mw, "参数错误", err.Error(), walk.MsgBoxIconWarning)
		return false
	}
	if a.content.Logging.File != "" {
		_ = os.MkdirAll(filepath.Dir(a.content.Logging.File), 0755)
	}
	b, err := json.MarshalIndent(a.content, "", "  ")
	if err == nil {
		err = os.WriteFile(a.configPath, b, 0600)
	}
	if err != nil {
		walk.MsgBox(a.mw, "保存失败", err.Error(), walk.MsgBoxIconError)
		return false
	}
	a.appendLog("设置已保存到 " + a.configPath)
	return true
}

func (a *app) start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return
	}
	if !a.saveConfig() {
		return
	}
	logger := slog.Default()
	conf, err := config.NewConfig(a.configPath, logger)
	if err != nil {
		a.failStart(err)
		return
	}
	drv, err := serverdriver.NewServer(conf, logger.With("component", "driver"))
	if err != nil {
		a.failStart(err)
		return
	}
	srv := ftpserverlib.NewFtpServer(drv)
	srv.Logger = logger.With("component", "server")
	if err := srv.Listen(); err != nil {
		a.failStart(err)
		return
	}
	a.driver, a.server, a.running = drv, srv, true
	a.startButton.SetEnabled(false)
	a.stopButton.SetEnabled(true)
	a.statusLabel.SetText("● 服务运行中")
	displayIP := a.selectedListenIP()
	if displayIP == "0.0.0.0" {
		displayIP = localAddress()
	}
	a.addressLabel.SetText("连接地址：ftp://" + displayIP + ":" + strconv.Itoa(int(a.ftpPort.Value())))
	a.appendLog("FTP服务已启动：" + a.content.ListenAddress)
	go func() {
		if err := srv.Serve(); err != nil && a.running {
			a.mw.Synchronize(func() { a.appendLog("服务异常停止：" + err.Error()); a.setStopped() })
		}
	}()
}

func (a *app) failStart(err error) {
	walk.MsgBox(a.mw, "启动失败", err.Error(), walk.MsgBoxIconError)
	a.appendLog("启动失败：" + err.Error())
}

func (a *app) stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running {
		return
	}
	a.driver.Stop()
	if err := a.server.Stop(); err != nil {
		a.appendLog("停止服务：" + err.Error())
	}
	a.setStopped()
	a.appendLog("FTP服务已停止")
}

func (a *app) setStopped() {
	a.running = false
	a.server = nil
	a.driver = nil
	a.startButton.SetEnabled(true)
	a.stopButton.SetEnabled(false)
	a.statusLabel.SetText("● 服务未启动")
	a.addressLabel.SetText("连接地址：尚未启动")
}

func (a *app) refreshUsers() {
	names := make([]string, len(a.users))
	for i, u := range a.users {
		names[i] = u.User
	}
	a.userList.SetModel(names)
	if len(names) > 0 {
		a.userList.SetCurrentIndex(0)
	}
}
func (a *app) selectUser() {
	i := a.userList.CurrentIndex()
	if i < 0 || i >= len(a.users) {
		return
	}
	u := a.users[i]
	a.username.SetText(u.User)
	a.password.SetText("")
	a.showPassword.SetChecked(false)
	a.password.SetPasswordMode(true)
	a.password.SetCueBanner("留空表示不修改现有密码")
	a.rootDir.SetText(u.Params["basePath"])
	a.readOnly.SetChecked(u.ReadOnly)
}
func (a *app) newUser() {
	a.userList.SetCurrentIndex(-1)
	a.username.SetText("")
	a.password.SetText("")
	a.showPassword.SetChecked(false)
	a.password.SetPasswordMode(true)
	a.password.SetCueBanner("请输入密码")
	a.rootDir.SetText("")
	a.readOnly.SetChecked(false)
	a.username.SetFocus()
}
func (a *app) saveCurrentUser() {
	name := strings.TrimSpace(a.username.Text())
	root := strings.TrimSpace(a.rootDir.Text())
	if name == "" || root == "" {
		walk.MsgBox(a.mw, "用户参数", "用户名和共享目录不能为空", walk.MsgBoxIconWarning)
		return
	}
	idx := a.userList.CurrentIndex()
	for i, u := range a.users {
		if u.User == name && i != idx {
			walk.MsgBox(a.mw, "用户参数", "用户名不能重复", walk.MsgBoxIconWarning)
			return
		}
	}
	pass := a.password.Text()
	if idx >= 0 && idx < len(a.users) && pass == "" {
		pass = a.users[idx].Pass
	}
	if pass == "" {
		walk.MsgBox(a.mw, "用户参数", "密码不能为空", walk.MsgBoxIconWarning)
		return
	}
	u := &confpar.Access{User: name, Pass: pass, Fs: "os", Params: map[string]string{"basePath": root}, ReadOnly: a.readOnly.Checked()}
	if idx >= 0 && idx < len(a.users) {
		a.users[idx] = u
	} else {
		a.users = append(a.users, u)
	}
	a.refreshUsers()
	a.appendLog("用户设置已更新：" + name)
}
func (a *app) deleteUser() {
	i := a.userList.CurrentIndex()
	if i < 0 || i >= len(a.users) {
		return
	}
	if walk.MsgBox(a.mw, "删除用户", "确定删除用户 "+a.users[i].User+"？", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	a.users = append(a.users[:i], a.users[i+1:]...)
	a.refreshUsers()
	a.newUser()
}
func (a *app) randomPassword() {
	password, err := generatePassword(16)
	if err != nil {
		walk.MsgBox(a.mw, "随机密码", "生成随机密码失败：\r\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.password.SetText(password)
	a.showPassword.SetChecked(true)
	a.password.SetTextSelection(0, -1)
	a.appendLog("随机密码已生成，请复制后提供给FTP用户")
}
func (a *app) changePasswordVisibility() {
	a.password.SetPasswordMode(!a.showPassword.Checked())
}
func (a *app) copyPassword() {
	password := a.password.Text()
	if password == "" {
		walk.MsgBox(a.mw, "复制密码", "当前没有可复制的密码。已有账户如需查看密码，请重新设置一个新密码。", walk.MsgBoxIconInformation)
		return
	}
	if err := walk.Clipboard().SetText(password); err != nil {
		walk.MsgBox(a.mw, "复制密码", "复制失败：\r\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.appendLog("密码已复制到剪贴板")
}
func (a *app) browseRoot() {
	dlg := new(walk.FileDialog)
	dlg.Title = "选择共享目录"
	if ok, _ := dlg.ShowBrowseFolder(a.mw); ok {
		a.rootDir.SetText(dlg.FilePath)
	}
}
func (a *app) browseFile(target *walk.LineEdit, title string) {
	dlg := new(walk.FileDialog)
	dlg.Title = title
	dlg.Filter = "PEM文件 (*.pem)|*.pem|所有文件 (*.*)|*.*"
	if ok, _ := dlg.ShowOpen(a.mw); ok {
		target.SetText(dlg.FilePath)
	}
}
func (a *app) browseLog(target *walk.LineEdit) {
	dlg := new(walk.FileDialog)
	dlg.Title = "选择日志文件"
	dlg.Filter = "日志文件 (*.log)|*.log|所有文件 (*.*)|*.*"
	if ok, _ := dlg.ShowSave(a.mw); ok {
		target.SetText(dlg.FilePath)
	}
}

func (a *app) showAdvanced() {
	var dlg *walk.Dialog
	var publicHost, certFile, keyFile, logFile *walk.LineEdit
	var tlsMode *walk.ComboBox
	var globalFTPLog, globalFileLog, hashPasswords, enableHash *walk.CheckBox
	var saveButton, cancelButton *walk.PushButton

	if err := (Dialog{
		AssignTo: &dlg,
		Title:    "高级设置",
		MinSize:  Size{Width: 560, Height: 360},
		Size:     Size{Width: 600, Height: 400},
		Layout:   VBox{},
		Children: []Widget{
			GroupBox{Title: "公网与安全", Layout: Grid{Columns: 3}, Children: []Widget{
				Label{Text: "公网IPv4："}, LineEdit{AssignTo: &publicHost, ColumnSpan: 2},
				Label{Text: "连接安全模式："}, ComboBox{AssignTo: &tlsMode, Model: []string{"普通FTP或FTPS", "强制显式FTPS", "隐式FTPS"}, ColumnSpan: 2},
				Label{Text: "TLS证书："}, LineEdit{AssignTo: &certFile}, PushButton{Text: "选择...", OnClicked: func() { a.browseFile(certFile, "选择TLS证书") }},
				Label{Text: "TLS私钥："}, LineEdit{AssignTo: &keyFile}, PushButton{Text: "选择...", OnClicked: func() { a.browseFile(keyFile, "选择TLS私钥") }},
			}},
			GroupBox{Title: "日志与其他", Layout: Grid{Columns: 3}, Children: []Widget{
				Label{Text: "日志文件："}, LineEdit{AssignTo: &logFile}, PushButton{Text: "选择...", OnClicked: func() { a.browseLog(logFile) }},
				Label{Text: "全局日志："}, CheckBox{AssignTo: &globalFTPLog, Text: "记录FTP命令"}, CheckBox{AssignTo: &globalFileLog, Text: "记录文件操作"},
				Label{Text: "其他选项："}, CheckBox{AssignTo: &hashPasswords, Text: "哈希保存密码"}, CheckBox{AssignTo: &enableHash, Text: "启用文件HASH"},
			}},
			Composite{Layout: HBox{}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &saveButton, Text: "确定"},
				PushButton{AssignTo: &cancelButton, Text: "取消", OnClicked: func() { dlg.Cancel() }},
			}},
		},
		DefaultButton: &saveButton,
		CancelButton:  &cancelButton,
	}).Create(a.mw); err != nil {
		walk.MsgBox(a.mw, "高级设置", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer dlg.Dispose()

	publicHost.SetText(a.content.PublicHost)
	logFile.SetText(a.content.Logging.File)
	globalFTPLog.SetChecked(a.content.Logging.FtpExchanges)
	globalFileLog.SetChecked(a.content.Logging.FileAccesses)
	hashPasswords.SetChecked(a.content.HashPlaintextPasswords)
	enableHash.SetChecked(a.content.Extensions.EnableHASH)
	switch a.content.TLSRequired {
	case "MandatoryEncryption":
		tlsMode.SetCurrentIndex(1)
	case "ImplicitEncryption":
		tlsMode.SetCurrentIndex(2)
	default:
		tlsMode.SetCurrentIndex(0)
	}
	if a.content.TLS != nil && a.content.TLS.ServerCert != nil {
		certFile.SetText(a.content.TLS.ServerCert.Cert)
		keyFile.SetText(a.content.TLS.ServerCert.Key)
	}

	saveButton.Clicked().Attach(func() {
		publicIP := strings.TrimSpace(publicHost.Text())
		if publicIP != "" && net.ParseIP(publicIP) == nil {
			walk.MsgBox(dlg, "参数错误", "公网地址必须是有效IP", walk.MsgBoxIconWarning)
			return
		}
		cert, key := strings.TrimSpace(certFile.Text()), strings.TrimSpace(keyFile.Text())
		if tlsMode.CurrentIndex() > 0 && (cert == "" || key == "") {
			walk.MsgBox(dlg, "参数错误", "启用强制FTPS时必须选择证书和私钥", walk.MsgBoxIconWarning)
			return
		}
		a.content.PublicHost = publicIP
		a.content.Logging = confpar.Logging{FtpExchanges: globalFTPLog.Checked(), FileAccesses: globalFileLog.Checked(), File: strings.TrimSpace(logFile.Text())}
		a.content.HashPlaintextPasswords = hashPasswords.Checked()
		a.content.Extensions.EnableHASH = enableHash.Checked()
		switch tlsMode.CurrentIndex() {
		case 1:
			a.content.TLSRequired = "MandatoryEncryption"
		case 2:
			a.content.TLSRequired = "ImplicitEncryption"
		default:
			a.content.TLSRequired = ""
		}
		if cert != "" || key != "" {
			a.content.TLS = &confpar.TLS{ServerCert: &confpar.ServerCert{Cert: cert, Key: key}}
		} else {
			a.content.TLS = nil
		}
		dlg.Accept()
		a.appendLog("高级设置已更新")
	})
	dlg.Run()
}

func (a *app) appendLog(s string) {
	if a.logView == nil {
		return
	}
	old := a.logView.Text()
	if old != "" {
		old += "\r\n"
	}
	a.logView.SetText(old + time.Now().Format("15:04:05") + "  " + s)
	a.logView.SetTextSelection(len([]rune(a.logView.Text())), len([]rune(a.logView.Text())))
}
func (a *app) about() {
	walk.MsgBox(a.mw, "关于", appName+"\r\nMake by 倚栏听雨\r\nVersion "+appVersion+"\r\n\r\nFTP核心基于 fclairamb/ftpserver\r\nLicensed under the MIT License", walk.MsgBoxIconInformation)
}
func (a *app) exit() { a.stop(); a.allowExit = true; a.mw.Close() }

func localAddress() string {
	return preferredLocalIPv4()
}

func preferredLocalIPv4() string {
	if conn, err := net.Dial("udp4", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP.To4() != nil && !addr.IP.IsLoopback() {
			return addr.IP.String()
		}
	}
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "0.0.0.0"
}

func (a *app) loadListenAddresses() {
	a.listenItems = []string{"所有网卡  (0.0.0.0)", "仅本机  (127.0.0.1)"}
	a.listenIPs = map[string]string{
		a.listenItems[0]: "0.0.0.0",
		a.listenItems[1]: "127.0.0.1",
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	seen := map[string]bool{"0.0.0.0": true, "127.0.0.1": true}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip == nil || ip.To4() == nil || seen[ip.String()] {
				continue
			}
			seen[ip.String()] = true
			item := fmt.Sprintf("%s  (%s)", iface.Name, ip.String())
			a.listenItems = append(a.listenItems, item)
			a.listenIPs[item] = ip.String()
		}
	}
}

func (a *app) selectedListenIP() string {
	text := strings.TrimSpace(a.listenIP.Text())
	if ip, ok := a.listenIPs[text]; ok {
		return ip
	}
	return text
}

func (a *app) listenDisplay(ip string) string {
	for item, itemIP := range a.listenIPs {
		if itemIP == ip {
			return item
		}
	}
	return ip
}
