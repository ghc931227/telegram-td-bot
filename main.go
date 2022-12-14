package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/downloader"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
	"mime"
)

var (
	taskQueue      *Queue
	errorTaskQueue *Queue
	taskQueueOpen  = true
	curThreadNum   *AtomicInt
	maxThreadNum   int
	mainCtx        context.Context
	api            *tg.Client
	sender         *message.Sender
	saveDir        string
	savedSize      int64
	mainDownloader *downloader.Downloader
)

func main() {
	apiIdStr := getEnvAny("apiId")
	apiHash := getEnvAny("apiHash")
	botToken := getEnvAny("botToken")
	onMessage := getEnvAny("onMessage")
	onChannelMessage := getEnvAny("onChannelMessage")
	userIdStr := getEnvAny("userId")
	channelIdStr := getEnvAny("channelId")
	saveDir := getEnvAny("saveDir")
	proxyIp := getEnvAny("proxyIp")
	proxyPort := getEnvAny("proxyPort")
	proxyAuth := getEnvAny("proxyAuth")
	proxyPwd := getEnvAny("proxyPwd")
	threadNumStr := getEnvAny("threadNum")

	flag.StringVar(&apiIdStr, "apiId", apiIdStr, "apiId")
	flag.StringVar(&apiHash, "apiHash", apiHash, "apiHash")
	flag.StringVar(&botToken, "botToken", botToken, "botToken")
	flag.StringVar(&onMessage, "onMessage", onMessage, "onMessage")
	flag.StringVar(&onChannelMessage, "onChannelMessage", onChannelMessage, "onChannelMessage")
	flag.StringVar(&userIdStr, "userId", userIdStr, "userId")
	flag.StringVar(&channelIdStr, "channelId", channelIdStr, "channelId")
	flag.StringVar(&saveDir, "saveDir", saveDir, "saveDir")
	flag.StringVar(&proxyIp, "proxyIp", proxyIp, "proxyIp")
	flag.StringVar(&proxyPort, "proxyPort", proxyPort, "proxyPort")
	flag.StringVar(&proxyAuth, "proxyAuth", proxyAuth, "proxyAuth")
	flag.StringVar(&proxyPwd, "proxyPwd", proxyPwd, "proxyPwd")
	flag.StringVar(&threadNumStr, "threadNum", threadNumStr, "threadNum")

	flag.Parse()

	apiId, err := strconv.Atoi(apiIdStr)
	if err != nil {
		consoleLog("Param Error: apiId must be a int value")
		return
	}
	if saveDir == "" {
		home, _ := user.Current()
		saveDir = home.HomeDir + "/Downloads"
	}
	if !strings.HasSuffix(saveDir, "/") {
		saveDir += "/"
	}
	if onMessage != "true" && onMessage != "false" {
		onMessage = "true"
	}
	if onChannelMessage != "true" && onChannelMessage != "false" {
		onChannelMessage = "true"
	}
	if channelIdStr == "" {
		channelIdStr = "0"
	}
	channelId, err := strconv.ParseInt(channelIdStr, 10, 64)
	if err != nil {
		consoleLog("Param Error: channelId must be a int value")
		return
	}
	if userIdStr == "" {
		userIdStr = "0"
	}
	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		consoleLog("Param Error: userId must be a int value")
		return
	}
	var dialer dcs.DialFunc
	if proxyIp != "" && proxyPort != "" {
		var auth *proxy.Auth
		if proxyAuth != "" || proxyPwd != "" {
			auth = &proxy.Auth{
				User:     proxyAuth,
				Password: proxyPwd,
			}
		}
		sock5, _ := proxy.SOCKS5("tcp", proxyIp+":"+proxyPort, auth, proxy.Direct)
		dc := sock5.(proxy.ContextDialer)
		dialer = dc.DialContext
	}
	if threadNumStr == "" {
		threadNumStr = "3"
	}
	threadNum, err := strconv.Atoi(threadNumStr)
	if err != nil {
		consoleLog("Param Error: threadNum must be a int value")
		return
	}

	consoleLogn(
		"apiId: ", apiIdStr,
		", apiHash: ", apiHash,
		", botToken: ", botToken,
		", onMessage: ", onMessage,
		", onChannelMessage: ", onChannelMessage,
		", userId: ", userIdStr,
		", channelId: ", channelIdStr,
		", saveDir: ", saveDir,
		", proxyIp: ", proxyIp,
		", proxyPort: ", proxyPort,
		", proxyAuth: ", proxyAuth,
		", proxyPwd: ", proxyPwd,
		", threadNum: ", threadNumStr,
	)

	clientOption := &ClientOption{
		apiId:            apiId,
		apiHash:          apiHash,
		botToken:         botToken,
		onChannelMessage: onChannelMessage,
		onMessage:        onMessage,
		userId:           userId,
		channelId:        channelId,
		saveDir:          saveDir,
		dialer:           dialer,
		threadNum:        threadNum,
	}

	err = listen(clientOption, context.Background())
	if err != nil {
		consoleLog(err.Error())
	}
}

func getEnvAny(names ...string) string {
	for _, n := range names {
		if val := os.Getenv(n); val != "" {
			return val
		}
	}
	return ""
}
func consoleLog(s string) {
	fmt.Println(time.Now().Format("[2006-01-02 15:04:05]: "), s)
}
func consoleLogn(s ...string) {
	fmt.Println(time.Now().Format("[2006-01-02 15:04:05]: "), s)
}

func listen(clientOption *ClientOption, ctx context.Context) error {
	dispatcher := tg.NewUpdateDispatcher()

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		if checkUser(update, clientOption.userId) {
			if onCommand(e, update) {
				return nil
			}
			if clientOption.onMessage == "true" {
				return onMessage(e, update)
			}
		}
		return nil
	})

	if clientOption.onChannelMessage == "true" {
		dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
			if checkChannel(update, clientOption.channelId) {
				return onMessage(e, update)
			}
			return nil
		})
	}

	logger, _ := zap.NewDevelopment()
	defer func() { _ = logger.Sync() }()

	client := telegram.NewClient(clientOption.apiId, clientOption.apiHash, telegram.Options{
		UpdateHandler: dispatcher,
		//Logger:        logger,
		Resolver: dcs.Plain(dcs.PlainOptions{
			Dial: clientOption.dialer,
		}),
	})

	mainCtx = ctx
	api = tg.NewClient(client)
	sender = message.NewSender(api)
	mainDownloader = downloader.NewDownloader()
	saveDir = clientOption.saveDir
	maxThreadNum = clientOption.threadNum
	taskQueue = new(Queue)
	errorTaskQueue = new(Queue)

	consoleLog("Starting service")
	return client.Run(ctx, func(ctx context.Context) error {
		consoleLog("Start to auth")
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		consoleLog("Start to login")
		if !status.Authorized {
			if _, err := client.Auth().Bot(ctx, clientOption.botToken); err != nil {
				return err
			}
		}
		go func() {
			startTaskQueue()
		}()
		consoleLog("Service ready")
		return telegram.RunUntilCanceled(ctx, client)
	})
}

func checkUser(update *tg.UpdateNewMessage, userId int64) bool {
	if userId == 0 {
		return true
	}
	msg, ok := update.GetMessage().(*tg.Message)
	if !ok {
		return false
	}
	if msg.PeerID.TypeName() == "peerUser" && msg.PeerID.(*tg.PeerUser).UserID == userId {
		return true
	}
	return false
}

func checkChannel(update *tg.UpdateNewChannelMessage, channelId int64) bool {
	if channelId == 0 {
		return true
	}
	msg, ok := update.GetMessage().(*tg.Message)
	if !ok {
		return false
	}
	if msg.PeerID.TypeName() == "peerChannel" && msg.PeerID.(*tg.PeerChannel).ChannelID == channelId {
		return true
	}
	return false
}

func onCommand(entities tg.Entities, update *tg.UpdateNewMessage) bool {
	msg, ok := update.GetMessage().(*tg.Message)
	if !ok {
		return true
	}
	textMsg := msg.Message
	switch textMsg {
	case "":
		return false
	case "/start":
		_, _ = sender.Reply(entities, update).Text(
			mainCtx,
			"Telegram Download Bot\nCommands:\n/status\t\tshow running status",
		)
		return true
	case "/status":
		_, _ = sender.Reply(entities, update).Text(
			mainCtx,
			getBotStatus(),
		)
		return true
	case "/retry":
		taskQueueOpen = false
		for {
			task := errorTaskQueue.Pop()
			if task == nil {
				break
			}
			taskQueue.Push(task)
		}
		taskQueueOpen = true
		return true
	case "/pause":
		taskQueueOpen = false
		return true
	case "/resume":
		taskQueueOpen = true
		return true
	default:
		if strings.HasPrefix(textMsg, "/set ") {
			configStr := strings.TrimPrefix(textMsg, "/set ")
			config := strings.Split(configStr, " ")
			if len(config) > 1 {
				configField := config[0]
				configValue := config[1]
				switch configField {
				case "maxThreadNum":
					configIntValue, err := strconv.Atoi(configValue)
					if err != nil {
						_, _ = sender.Reply(entities, update).Text(mainCtx, "wrong param: "+configValue)
					} else {
						maxThreadNum = configIntValue
						_, _ = sender.Reply(entities, update).Text(mainCtx, "maxThreadNum changed: "+configValue)
					}
					break
				case "saveDir":
					fi, err := os.Stat(configValue)
					if err == nil && fi.IsDir() {
						if !strings.HasSuffix(configValue, "/") {
							configValue += "/"
						}
						if saveDir != configValue {
							saveDir = configValue
							savedSize = 0
							_, _ = sender.Reply(entities, update).Text(mainCtx, "saveDir path changed: "+configValue)
						}
					} else {
						_, _ = sender.Reply(entities, update).Text(mainCtx, "wrong path: "+configValue)
					}
					break
				default:
					return false
				}
				return true
			}
		}
		if strings.HasPrefix(textMsg, "/run ") {
			configStr := strings.TrimPrefix(textMsg, "/run ")
			config := strings.Split(configStr, " ")
			if len(config) >= 1 {
				configValue := config[0]
				fi, err := os.Stat(configValue)
				if err == nil && !fi.IsDir() && (configValue == "run.sh" || configValue == "run.cmd") {
					go func() {
						curDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
						curPath := strings.Replace(curDir, "\\", "/", -1)
						cmd := exec.Command(curPath+"/"+configValue, config...)

						var out bytes.Buffer
						cmd.Stdout = &out

						err = cmd.Run()

						result := "success"
						if err != nil {
							result = "faild"
							consoleLog(fmt.Sprintf("Command execute: %s", err))
						}
						reply := out.String()
						if reply == "" {
							reply = "no stdout return"
						}
						_, _ = sender.Reply(entities, update).Text(mainCtx, fmt.Sprintf("command execute %s:\n%s", result, out.String()))
					}()
				} else {
					_, _ = sender.Reply(entities, update).Text(mainCtx, "wrong path: "+configValue)
				}
				return true
			}
		}
	}
	return false
}

func onMessage(entities tg.Entities, messageUpdate message.AnswerableMessageUpdate) error {
	msg, ok := messageUpdate.GetMessage().(*tg.Message)
	if !ok {
		return nil
	}
	document, gotDocument := getDocument(msg)
	photo, gotPhoto := getPhoto(msg)
	if !gotDocument && !gotPhoto {
		return nil
	}

	var filename string
	if document != nil {
		filename = "[" + strconv.Itoa(msg.GetID()) + "] " + getDocumentFileName(document)
		downloadTask := &DownloadTask{
			document:   document,
			photo:      nil,
			fineName:   filename,
			entities:   entities,
			newMessage: messageUpdate,
		}
		taskQueue.Push(downloadTask)
	}
	if photo != nil {
		filename = "[" + strconv.Itoa(msg.GetID()) + "] " + getPhotoFileName(photo)
		downloadTask := &DownloadTask{
			document:   nil,
			photo:      photo,
			fineName:   filename,
			entities:   entities,
			newMessage: messageUpdate,
		}
		taskQueue.Push(downloadTask)
	}
	consoleLog("Got file: " + filename)
	return nil
}

func startTaskQueue() {
	curThreadNum = new(AtomicInt)
	for {
		runningTasks := curThreadNum.Value()
		if time.Now().Second() == 0 {
			consoleLog(getBotStatus())
		}
		if taskQueueOpen {
			if runningTasks < maxThreadNum {
				downloadTask := taskQueue.Pop()
				if downloadTask != nil {
					go func() {
						curThreadNum.Higher()
						success := downloadFile(downloadTask)
						for !success && downloadTask.retryNum < 3 {
							downloadTask.retryNum += 1
							success = downloadFile(downloadTask)
						}
						curThreadNum.Lower()
						if !success {
							errorTaskQueue.Push(downloadTask)
						}
					}()
				}
			}
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func getDocument(msg *tg.Message) (*tg.Document, bool) {
	mediaDocument, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil, false
	}
	document, ok := mediaDocument.Document.(*tg.Document)
	if !ok {
		return nil, false
	}
	return document, true
}

func getPhoto(msg *tg.Message) (*tg.Photo, bool) {
	mediaPhoto, ok := msg.Media.(*tg.MessageMediaPhoto)
	if !ok {
		return nil, false
	}
	photo, ok := mediaPhoto.Photo.(*tg.Photo)
	if !ok {
		return nil, false
	}
	return photo, true
}

func getDocumentFileName(document *tg.Document) string {
	name := ""
	for _, attr := range document.Attributes {
		switch attr := attr.(type) {
		case *tg.DocumentAttributeFilename:
			name = attr.FileName
			break
		case *tg.DocumentAttributeAudio:
			name = fmt.Sprintf("%s.%s", attr.Title, mime2ext(document.GetMimeType()))
			break
		}
	}
	if name == "" {
		name = fmt.Sprintf("%d%s", document.GetID(), mime2ext(document.GetMimeType()))
	}
	return name
}

func getPhotoFileName(photo *tg.Photo) string {
	name := fmt.Sprintf("%d%s", photo.GetID(), ".jpg")
	return name
}

func mime2ext(s string) string {
	res, err := mime.ExtensionsByType(s)
	if err != nil {
		return ""
	}
	if len(res) == 0 {
		return ""
	}
	return res[0]
}

func downloadFile(task *DownloadTask) bool {
	if task.document == nil && task.photo == nil {
		return true
	}
	startTime := time.Now()
	document := task.document
	photo := task.photo
	var fileSize int64
	var err error
	if task.retryNum != 0 {
		saveLog := fmt.Sprintf("Retry download [%d]: %s", task.retryNum, task.fineName)
		consoleLog(saveLog)
		_, _ = sender.Reply(task.entities, task.newMessage).Text(mainCtx, saveLog)
	}
	if document != nil {
		fileSize = document.Size
		fileSizeStr := strconv.FormatFloat(float64(fileSize)/float64(1024)/float64(1024), 'f', 2, 64)
		consoleLog("Start download: " + task.fineName + ", Size: " + fileSizeStr + " mb")
		builder := mainDownloader.Download(api, &tg.InputDocumentFileLocation{
			ID:            document.GetID(),
			AccessHash:    document.GetAccessHash(),
			FileReference: document.GetFileReference(),
		})
		_, err = builder.ToPath(context.Background(), saveDir+task.fineName)
	} else if photo != nil {
		fileSize = 0
		consoleLog("Start download: " + task.fineName)
		builder := mainDownloader.Download(api, &tg.InputPhotoFileLocation{
			ID:            photo.GetID(),
			AccessHash:    photo.GetAccessHash(),
			FileReference: photo.GetFileReference(),
			ThumbSize:     "y",
		})
		_, err = builder.ToPath(context.Background(), saveDir+task.fineName)
	}
	if err != nil {
		saveLog := fmt.Sprintf("Download error: [%s] %s", task.fineName, err)
		consoleLog(saveLog)
		if task.retryNum >= 3 {
			_, _ = sender.Reply(task.entities, task.newMessage).Text(mainCtx, saveLog)
		}
		return false
	} else {
		savedSize += fileSize
		costTime := time.Now().Sub(startTime).Milliseconds() / 1000
		saveLog := fmt.Sprintf("Download success: [%s], %s", task.fineName, getDownloadAnalyzation(costTime, fileSize))
		consoleLog(saveLog)
		_, _ = sender.Reply(task.entities, task.newMessage).Text(mainCtx, saveLog)
	}
	return true
}

func getDownloadAnalyzation(costTime int64, fileSize int64) string {
	if costTime == 0 {
		costTime = 1
	}
	costTimeStr := strconv.FormatInt(costTime, 10) + " seconds"
	if costTime/60 != 0 {
		costTimeStr = strconv.FormatInt(costTime/60, 10) + " minutes"
	}
	if fileSize == 0 {
		return fmt.Sprintf("Cost: %s", costTimeStr)
	}
	downloadSpeed := fileSize / costTime
	downloadSpeedStr := strconv.FormatFloat(float64(downloadSpeed)/float64(1024*1024), 'f', 2, 64)
	return fmt.Sprintf("Cost: %s, Speed: %s mb/s", costTimeStr, downloadSpeedStr)
}

func getBotStatus() string {
	if !taskQueueOpen {
		return "Paused"
	}
	return fmt.Sprintf(
		"Running tasks: %d, Waiting tasks: %d, Error tasks %d, %s GB downloaded.",
		curThreadNum.Value(),
		taskQueue.Len(),
		errorTaskQueue.Len(),
		strconv.FormatFloat(float64(savedSize)/float64(1024*1024*1024), 'f', 2, 64),
	)
}
